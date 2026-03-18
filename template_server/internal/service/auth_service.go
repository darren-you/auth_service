package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	authguest "github.com/darren-you/auth_service/guest"
	authphone "github.com/darren-you/auth_service/phone"
	authapple "github.com/darren-you/auth_service/provider/apple"
	authwechat "github.com/darren-you/auth_service/provider/wechat"
	"github.com/darren-you/auth_service/session"
	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/dto"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errors"
	"github.com/darren-you/auth_service/template_server/internal/model"
	mysqlRepo "github.com/darren-you/auth_service/template_server/internal/repository/mysql"
	redisRepo "github.com/darren-you/auth_service/template_server/internal/repository/redis"
	"github.com/darren-you/auth_service/template_server/pkg/logger"
	"github.com/google/uuid"
)

const maxStateStoreRetry = 3

type AuthService interface {
	SyncCatalog(ctx context.Context) error
	SessionConfig() session.Config
	GetLoginURL(ctx context.Context, tenantKey string, provider string, clientType string) (*dto.LoginURLResponse, error)
	RegisterPassword(ctx context.Context, req dto.PasswordRegisterRequest) (*dto.SessionResponse, error)
	ProviderCallback(ctx context.Context, provider string, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error)
	SendPhoneCaptcha(ctx context.Context, req dto.PhoneCaptchaSendRequest) (*dto.PhoneCaptchaSendResponse, error)
	IssueGuestDeviceID(ctx context.Context, req dto.GuestDeviceIDRequest) (*dto.GuestDeviceIDResponse, error)
	Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.SessionResponse, error)
	Logout(ctx context.Context, req dto.LogoutRequest) error
	GetUserProfileByID(ctx context.Context, userID uint) (*dto.AuthUserResponse, error)
}

type authService struct {
	mysqlRepo     mysqlRepo.AuthRepository
	redisRepo     redisRepo.KVRepository
	authConfig    config.AuthConfig
	sessionConfig session.Config
}

type tenantRuntimeConfig struct {
	BridgeBaseURL string
	BridgeAuthKey string
}

type businessUserProfile struct {
	UserID      uint   `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type businessBridgeRequest struct {
	TenantKey       string `json:"tenant_key"`
	Provider        string `json:"provider"`
	Action          string `json:"action,omitempty"`
	ClientType      string `json:"client_type"`
	Username        string `json:"username,omitempty"`
	Email           string `json:"email,omitempty"`
	Password        string `json:"password,omitempty"`
	OpenID          string `json:"open_id,omitempty"`
	UnionID         string `json:"union_id,omitempty"`
	AppleUserID     string `json:"apple_user_id,omitempty"`
	Phone           string `json:"phone,omitempty"`
	DeviceID        string `json:"device_id,omitempty"`
	DisplayName     string `json:"display_name,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	CurrentUserID   uint   `json:"current_user_id,omitempty"`
	CurrentUserRole string `json:"current_user_role,omitempty"`
}

type bridgeEnvelope struct {
	Code int                 `json:"code"`
	Msg  string              `json:"msg"`
	Data businessUserProfile `json:"data"`
}

type sessionMetadata struct {
	TenantKey   string `json:"tenant_key"`
	Provider    string `json:"provider"`
	ClientType  string `json:"client_type"`
	TokenUserID uint   `json:"token_user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

func NewAuthService(mysqlRepo mysqlRepo.AuthRepository, redisRepo redisRepo.KVRepository, authConfig config.AuthConfig, sessionConfig session.Config) AuthService {
	return &authService{
		mysqlRepo:     mysqlRepo,
		redisRepo:     redisRepo,
		authConfig:    authConfig,
		sessionConfig: sessionConfig,
	}
}

func (s *authService) SyncCatalog(ctx context.Context) error {
	return s.mysqlRepo.SyncCatalog(ctx, s.authConfig.Tenants)
}

func (s *authService) SessionConfig() session.Config {
	return s.sessionConfig
}

func (s *authService) GetLoginURL(ctx context.Context, tenantKey string, provider string, clientType string) (*dto.LoginURLResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, tenantKey, provider, clientType)
	if err != nil {
		return nil, err
	}

	if provider != "wechat" {
		return nil, appErrors.ErrUnsupportedProvider
	}

	state, err := s.allocateState(ctx, tenant.TenantKey, provider, providerConfig.ClientType)
	if err != nil {
		return nil, err
	}

	client := authwechat.NewClient(authwechat.Config{
		AppID:          providerConfig.AppID,
		AppSecret:      providerConfig.AppSecret,
		WebRedirectURI: providerConfig.RedirectURI,
		LoginScope:     providerConfig.Scope,
	})
	loginURL, err := client.BuildWebLoginURL(state)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, appErrors.ErrConfigInvalid.Message, err)
	}

	return &dto.LoginURLResponse{
		TenantKey:  tenant.TenantKey,
		Provider:   provider,
		ClientType: providerConfig.ClientType,
		LoginURL:   loginURL,
		State:      state,
	}, nil
}

func (s *authService) ProviderCallback(ctx context.Context, provider string, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error) {
	switch normalize(provider) {
	case "wechat":
		return s.loginWithWeChat(ctx, req)
	case "apple":
		return s.loginWithApple(ctx, req)
	case "password":
		return s.loginWithPassword(ctx, req)
	case "phone":
		return s.loginWithPhone(ctx, req)
	case "guest":
		return s.loginWithGuest(ctx, req)
	default:
		return nil, appErrors.ErrUnsupportedProvider
	}
}

func (s *authService) RegisterPassword(ctx context.Context, req dto.PasswordRegisterRequest) (*dto.SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "password", req.ClientType)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	if username == "" || email == "" || password == "" {
		return nil, appErrors.ErrBadRequest
	}

	businessUser, err := s.syncBusinessUser(ctx, tenant.TenantKey, "password", providerConfig.ClientType, businessBridgeRequest{
		Action:      "register",
		Username:    username,
		Email:       email,
		Password:    password,
		DisplayName: firstNonEmpty(req.DisplayName, username),
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return nil, err
	}

	return s.issuePasswordSession(ctx, tenant, providerConfig, businessUser, username, req.AvatarURL)
}

func (s *authService) SendPhoneCaptcha(ctx context.Context, req dto.PhoneCaptchaSendRequest) (*dto.PhoneCaptchaSendResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "phone", req.ClientType)
	if err != nil {
		return nil, err
	}

	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		return nil, appErrors.ErrBadRequest
	}

	if providerConfig.TestPhone == "" || phone != providerConfig.TestPhone {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "phone sender is not configured for non-test phone", nil)
	}

	phoneService := authphone.NewService(
		phoneCaptchaStoreAdapter{
			repo:   s.redisRepo,
			prefix: s.phoneCaptchaPrefix(tenant.TenantKey, providerConfig.ClientType),
		},
		nil,
		authphone.Config{
			TestPhone:      providerConfig.TestPhone,
			TestCaptcha:    providerConfig.TestCaptcha,
			TestCaptchaKey: providerConfig.TestCaptchaKey,
			TTL:            time.Duration(s.authConfig.PhoneCaptchaTTLSecond) * time.Second,
			CaptchaLength:  4,
		},
	)

	result, err := phoneService.Send(ctx, phone)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	return &dto.PhoneCaptchaSendResponse{
		TenantKey:  tenant.TenantKey,
		ClientType: providerConfig.ClientType,
		CaptchaKey: result.CaptchaKey,
		ExpiresIn:  result.ExpiresIn,
	}, nil
}

func (s *authService) IssueGuestDeviceID(ctx context.Context, req dto.GuestDeviceIDRequest) (*dto.GuestDeviceIDResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "guest", req.ClientType)
	if err != nil {
		return nil, err
	}

	guestService := authguest.NewService(
		guestDeviceStoreAdapter{
			repo:   s.redisRepo,
			prefix: s.guestDevicePrefix(tenant.TenantKey, providerConfig.ClientType),
		},
		time.Duration(s.authConfig.GuestDeviceTTLSecond)*time.Second,
	)

	result, err := guestService.IssueVirtualDeviceID(ctx)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	return &dto.GuestDeviceIDResponse{
		TenantKey:  tenant.TenantKey,
		ClientType: providerConfig.ClientType,
		DeviceID:   result.DeviceID,
		ExpiresIn:  result.ExpiresIn,
	}, nil
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.SessionResponse, error) {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return nil, appErrors.ErrBadRequest
	}

	claims, err := session.ParseRefreshToken(refreshToken, s.sessionConfig)
	if err != nil {
		if err == session.ErrExpiredToken {
			return nil, appErrors.ErrTokenExpired
		}
		return nil, appErrors.ErrTokenInvalid
	}

	sessionRecord, err := s.mysqlRepo.FindSessionByHash(ctx, hashToken(refreshToken))
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if sessionRecord == nil {
		return nil, appErrors.ErrSessionRevoked
	}
	if sessionRecord.RevokedAt != nil {
		return nil, appErrors.ErrSessionRevoked
	}
	if time.Now().After(sessionRecord.ExpiresAt) {
		return nil, appErrors.ErrTokenExpired
	}

	metadata, err := parseSessionMetadata(sessionRecord.MetadataJSON)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if metadata.TokenUserID == 0 || metadata.TokenUserID != claims.UserID {
		return nil, appErrors.ErrSessionRevoked
	}

	tenant, err := s.mysqlRepo.FindTenantByID(ctx, sessionRecord.TenantID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil {
		return nil, appErrors.ErrTenantNotFound
	}

	if err := s.mysqlRepo.RevokeSessionByHash(ctx, sessionRecord.RefreshTokenHash, time.Now()); err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	return s.issueSession(ctx, tenant, sessionRecord.AuthUserID, sessionRecord.Provider, sessionRecord.ClientType, &businessUserProfile{
		UserID:      metadata.TokenUserID,
		DisplayName: metadata.DisplayName,
		AvatarURL:   metadata.AvatarURL,
		Role:        metadata.Role,
		Status:      metadata.Status,
	})
}

func (s *authService) Logout(ctx context.Context, req dto.LogoutRequest) error {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return appErrors.ErrBadRequest
	}

	if _, err := session.ParseRefreshTokenIgnoringExpiry(refreshToken, s.sessionConfig); err != nil {
		return appErrors.ErrTokenInvalid
	}

	if err := s.mysqlRepo.RevokeSessionByHash(ctx, hashToken(refreshToken), time.Now()); err != nil {
		return appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	return nil
}

func (s *authService) GetUserProfileByID(ctx context.Context, userID uint) (*dto.AuthUserResponse, error) {
	user, err := s.mysqlRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if user == nil {
		return nil, appErrors.ErrUnauthorized
	}

	tenant, err := s.mysqlRepo.FindTenantByID(ctx, user.TenantID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil {
		return nil, appErrors.ErrTenantNotFound
	}

	profile := buildUserResponse(user, tenant)
	return &profile, nil
}

func (s *authService) loginWithWeChat(ctx context.Context, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "wechat", req.ClientType)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(req.Code)
	state := strings.TrimSpace(req.State)
	if code == "" || state == "" {
		return nil, appErrors.ErrBadRequest
	}

	ok, err := s.redisRepo.Consume(ctx, s.stateKey(tenant.TenantKey, "wechat", providerConfig.ClientType, state))
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if !ok {
		return nil, appErrors.ErrWeChatStateInvalid
	}

	client := authwechat.NewClient(authwechat.Config{
		AppID:          providerConfig.AppID,
		AppSecret:      providerConfig.AppSecret,
		WebRedirectURI: providerConfig.RedirectURI,
		LoginScope:     providerConfig.Scope,
	})
	oauthToken, err := client.ExchangeCode(ctx, code)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}
	oauthToken, err = client.EnsureAccessTokenValid(ctx, oauthToken)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	userInfo, err := client.FetchUserInfo(ctx, oauthToken.AccessToken, oauthToken.OpenID)
	if err != nil {
		logger.Warnf("Fetch wechat user info failed: %v", err)
	}

	displayName := defaultDisplayName("wechat", oauthToken.OpenID)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if userInfo != nil {
		if strings.TrimSpace(userInfo.Nickname) != "" {
			displayName = strings.TrimSpace(userInfo.Nickname)
		}
		if strings.TrimSpace(userInfo.HeadImgURL) != "" {
			avatarURL = strings.TrimSpace(userInfo.HeadImgURL)
		}
	}

	user, err := s.upsertIdentityUser(ctx, tenant, providerConfig, "wechat", oauthToken.OpenID, oauthToken.UnionID, displayName, avatarURL, "user", marshalJSON(userInfo))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(ctx, tenant.TenantKey, "wechat", providerConfig.ClientType, businessBridgeRequest{
		OpenID:          oauthToken.OpenID,
		UnionID:         oauthToken.UnionID,
		DisplayName:     displayName,
		AvatarURL:       avatarURL,
		CurrentUserID:   req.CurrentUserID,
		CurrentUserRole: req.CurrentUserRole,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, tenant, user.ID, "wechat", providerConfig.ClientType, businessUser)
}

func (s *authService) loginWithApple(ctx context.Context, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "apple", req.ClientType)
	if err != nil {
		return nil, err
	}

	authCode := firstNonEmpty(req.AuthorizationCode, req.Code)
	if authCode == "" {
		return nil, appErrors.ErrBadRequest
	}

	client := authapple.NewClient(authapple.Config{
		SigningKey: providerConfig.SigningKey,
		TeamID:     providerConfig.TeamID,
		ClientID:   providerConfig.ClientID,
		KeyID:      providerConfig.KeyID,
	})
	validationResp, err := client.VerifyAuthorizationCode(ctx, authCode)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	uniqueID, err := client.GetUniqueIDFromIDToken(validationResp.IDToken)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	displayName := firstNonEmpty(req.DisplayName, defaultDisplayName("apple", uniqueID))
	user, err := s.upsertIdentityUser(ctx, tenant, providerConfig, "apple", uniqueID, "", displayName, req.AvatarURL, "user", marshalJSON(validationResp))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(ctx, tenant.TenantKey, "apple", providerConfig.ClientType, businessBridgeRequest{
		AppleUserID:     uniqueID,
		DisplayName:     displayName,
		AvatarURL:       req.AvatarURL,
		CurrentUserID:   req.CurrentUserID,
		CurrentUserRole: req.CurrentUserRole,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, tenant, user.ID, "apple", providerConfig.ClientType, businessUser)
}

func (s *authService) loginWithPassword(ctx context.Context, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "password", req.ClientType)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return nil, appErrors.ErrBadRequest
	}

	businessUser, err := s.syncBusinessUser(ctx, tenant.TenantKey, "password", providerConfig.ClientType, businessBridgeRequest{
		Action:      "login",
		Username:    username,
		Password:    password,
		DisplayName: firstNonEmpty(req.DisplayName, username),
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return nil, err
	}

	return s.issuePasswordSession(ctx, tenant, providerConfig, businessUser, username, req.AvatarURL)
}

func (s *authService) loginWithPhone(ctx context.Context, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "phone", req.ClientType)
	if err != nil {
		return nil, err
	}

	phone := strings.TrimSpace(req.Phone)
	if phone == "" || strings.TrimSpace(req.Captcha) == "" || strings.TrimSpace(req.CaptchaKey) == "" {
		return nil, appErrors.ErrBadRequest
	}

	phoneService := authphone.NewService(
		phoneCaptchaStoreAdapter{
			repo:   s.redisRepo,
			prefix: s.phoneCaptchaPrefix(tenant.TenantKey, providerConfig.ClientType),
		},
		nil,
		authphone.Config{
			TestPhone:      providerConfig.TestPhone,
			TestCaptcha:    providerConfig.TestCaptcha,
			TestCaptchaKey: providerConfig.TestCaptchaKey,
			TTL:            time.Duration(s.authConfig.PhoneCaptchaTTLSecond) * time.Second,
			CaptchaLength:  4,
		},
	)
	if err := phoneService.Verify(ctx, authphone.VerifyRequest{
		Phone:      phone,
		Captcha:    req.Captcha,
		CaptchaKey: req.CaptchaKey,
	}); err != nil {
		return nil, appErrors.ErrCaptchaInvalid
	}

	displayName := firstNonEmpty(req.DisplayName, maskPhone(phone))
	user, err := s.upsertIdentityUser(ctx, tenant, providerConfig, "phone", phone, "", displayName, req.AvatarURL, "user", marshalJSON(map[string]string{"phone": phone}))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(ctx, tenant.TenantKey, "phone", providerConfig.ClientType, businessBridgeRequest{
		Phone:       phone,
		DisplayName: displayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, tenant, user.ID, "phone", providerConfig.ClientType, businessUser)
}

func (s *authService) loginWithGuest(ctx context.Context, req dto.ProviderCallbackRequest) (*dto.SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(ctx, req.TenantKey, "guest", req.ClientType)
	if err != nil {
		return nil, err
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, appErrors.ErrBadRequest
	}

	guestService := authguest.NewService(
		guestDeviceStoreAdapter{
			repo:   s.redisRepo,
			prefix: s.guestDevicePrefix(tenant.TenantKey, providerConfig.ClientType),
		},
		time.Duration(s.authConfig.GuestDeviceTTLSecond)*time.Second,
	)
	valid, err := guestService.IsValid(ctx, deviceID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if !valid {
		return nil, appErrors.ErrUnauthorized
	}

	displayName := firstNonEmpty(req.DisplayName, authguest.UsernameFromDeviceID(deviceID))
	user, err := s.upsertIdentityUser(ctx, tenant, providerConfig, "guest", deviceID, "", displayName, req.AvatarURL, "guest", marshalJSON(map[string]string{"device_id": deviceID}))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(ctx, tenant.TenantKey, "guest", providerConfig.ClientType, businessBridgeRequest{
		DeviceID:    deviceID,
		DisplayName: displayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, tenant, user.ID, "guest", providerConfig.ClientType, businessUser)
}

func (s *authService) issuePasswordSession(
	ctx context.Context,
	tenant *model.AuthTenant,
	providerConfig *model.AuthProviderConfig,
	businessUser *businessUserProfile,
	username string,
	avatarURL string,
) (*dto.SessionResponse, error) {
	if businessUser == nil || businessUser.UserID == 0 {
		return nil, appErrors.ErrAuthFailed
	}

	subject := strconv.FormatUint(uint64(businessUser.UserID), 10)
	displayName := firstNonEmpty(businessUser.DisplayName, username)
	normalizedAvatarURL := firstNonEmpty(businessUser.AvatarURL, avatarURL)
	user, err := s.upsertIdentityUser(
		ctx,
		tenant,
		providerConfig,
		"password",
		subject,
		"",
		displayName,
		normalizedAvatarURL,
		firstNonEmpty(businessUser.Role, "user"),
		marshalJSON(map[string]string{"username": username}),
	)
	if err != nil {
		return nil, err
	}

	if businessUser.DisplayName == "" {
		businessUser.DisplayName = displayName
	}
	if businessUser.AvatarURL == "" {
		businessUser.AvatarURL = normalizedAvatarURL
	}
	return s.issueSession(ctx, tenant, user.ID, "password", providerConfig.ClientType, businessUser)
}

func (s *authService) resolveTenantAndProvider(ctx context.Context, tenantKey string, provider string, clientType string) (*model.AuthTenant, *model.AuthProviderConfig, error) {
	tenant, err := s.mysqlRepo.FindTenantByKey(ctx, normalize(tenantKey))
	if err != nil {
		return nil, nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil || !tenant.Enabled {
		return nil, nil, appErrors.ErrTenantNotFound
	}

	providerConfig, err := s.mysqlRepo.FindProviderConfig(ctx, tenant.ID, normalize(provider), normalize(clientType))
	if err != nil {
		return nil, nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if providerConfig == nil || !providerConfig.Enabled {
		return nil, nil, appErrors.ErrProviderNotEnabled
	}

	return tenant, providerConfig, nil
}

func (s *authService) upsertIdentityUser(
	ctx context.Context,
	tenant *model.AuthTenant,
	providerConfig *model.AuthProviderConfig,
	provider string,
	subject string,
	unionID string,
	displayName string,
	avatarURL string,
	role string,
	profileJSON string,
) (*model.AuthUser, error) {
	user, identity, err := s.mysqlRepo.FindUserByIdentity(ctx, tenant.ID, provider, subject)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	now := time.Now()
	if user == nil {
		user = &model.AuthUser{
			TenantID:    tenant.ID,
			DisplayName: firstNonEmpty(displayName, defaultDisplayName(provider, subject)),
			AvatarURL:   strings.TrimSpace(avatarURL),
			Role:        role,
			Status:      "active",
			LastLoginAt: &now,
		}
		identity = &model.AuthIdentity{
			TenantID:        tenant.ID,
			Provider:        provider,
			ClientType:      providerConfig.ClientType,
			ProviderSubject: subject,
			UnionID:         strings.TrimSpace(unionID),
			ProfileJSON:     profileJSON,
		}
		if err := s.mysqlRepo.CreateUserWithIdentity(ctx, user, identity); err != nil {
			return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
		}
		return user, nil
	}

	updatedDisplayName := user.DisplayName
	if strings.TrimSpace(displayName) != "" {
		updatedDisplayName = strings.TrimSpace(displayName)
	}
	updatedAvatarURL := user.AvatarURL
	if strings.TrimSpace(avatarURL) != "" {
		updatedAvatarURL = strings.TrimSpace(avatarURL)
	}
	if err := s.mysqlRepo.UpdateUserLogin(ctx, user.ID, updatedDisplayName, updatedAvatarURL, now); err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if identity != nil {
		if err := s.mysqlRepo.UpdateIdentity(ctx, identity.ID, providerConfig.ClientType, strings.TrimSpace(unionID), profileJSON); err != nil {
			return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
		}
	}
	user.DisplayName = updatedDisplayName
	user.AvatarURL = updatedAvatarURL
	user.LastLoginAt = &now
	return user, nil
}

func (s *authService) issueSession(ctx context.Context, tenant *model.AuthTenant, authUserID uint, provider string, clientType string, businessUser *businessUserProfile) (*dto.SessionResponse, error) {
	if businessUser == nil || businessUser.UserID == 0 {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "business bridge user is empty", nil)
	}

	normalizedProfile := normalizeBusinessProfile(businessUser)
	tokenPair, err := session.GenerateTokenPairWithProfile(
		normalizedProfile.UserID,
		normalizedProfile.DisplayName,
		"",
		normalizedProfile.Role,
		tenant.TenantKey,
		normalizedProfile.AvatarURL,
		normalizedProfile.Status,
		s.sessionConfig,
	)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	now := time.Now()
	authSession := &model.AuthSession{
		TenantID:         tenant.ID,
		AuthUserID:       authUserID,
		Provider:         provider,
		ClientType:       clientType,
		RefreshTokenHash: hashToken(tokenPair.RefreshToken),
		ExpiresAt:        now.Add(s.sessionConfig.RefreshExpiry),
		LastSeenAt:       &now,
		MetadataJSON: marshalJSON(sessionMetadata{
			TenantKey:   tenant.TenantKey,
			Provider:    provider,
			ClientType:  clientType,
			TokenUserID: normalizedProfile.UserID,
			DisplayName: normalizedProfile.DisplayName,
			AvatarURL:   normalizedProfile.AvatarURL,
			Role:        normalizedProfile.Role,
			Status:      normalizedProfile.Status,
		}),
	}
	if err := s.mysqlRepo.CreateSession(ctx, authSession); err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	return &dto.SessionResponse{
		TenantKey:    tenant.TenantKey,
		Provider:     provider,
		ClientType:   clientType,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		User: dto.AuthUserResponse{
			ID:          normalizedProfile.UserID,
			TenantKey:   tenant.TenantKey,
			DisplayName: normalizedProfile.DisplayName,
			AvatarURL:   normalizedProfile.AvatarURL,
			Role:        normalizedProfile.Role,
			Status:      normalizedProfile.Status,
		},
	}, nil
}

func (s *authService) resolveTenantRuntimeConfig(tenantKey string) (*tenantRuntimeConfig, error) {
	normalizedTenantKey := normalize(tenantKey)
	for _, tenant := range s.authConfig.Tenants {
		if normalize(tenant.Key) != normalizedTenantKey {
			continue
		}
		return &tenantRuntimeConfig{
			BridgeBaseURL: strings.TrimRight(strings.TrimSpace(tenant.BridgeBaseURL), "/"),
			BridgeAuthKey: strings.TrimSpace(tenant.BridgeAuthKey),
		}, nil
	}
	return nil, appErrors.ErrTenantNotFound
}

func (s *authService) syncBusinessUser(ctx context.Context, tenantKey string, provider string, clientType string, req businessBridgeRequest) (*businessUserProfile, error) {
	runtimeCfg, err := s.resolveTenantRuntimeConfig(tenantKey)
	if err != nil {
		return nil, err
	}
	if runtimeCfg.BridgeBaseURL == "" || runtimeCfg.BridgeAuthKey == "" {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "business bridge is not configured", nil)
	}

	req.TenantKey = normalize(tenantKey)
	req.Provider = normalize(provider)
	req.ClientType = normalize(clientType)
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, runtimeCfg.BridgeBaseURL+"/api/v1/internal/auth/sync", bytes.NewReader(payload))
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Auth-Service-Key", runtimeCfg.BridgeAuthKey)

	httpResp, err := (&http.Client{Timeout: 8 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}
	defer httpResp.Body.Close()

	var envelope bridgeEnvelope
	if err := json.NewDecoder(httpResp.Body).Decode(&envelope); err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices || envelope.Code != 200 {
		msg := strings.TrimSpace(envelope.Msg)
		if msg == "" {
			msg = "business bridge rejected auth sync"
		}
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, msg, nil)
	}

	profile := normalizeBusinessProfile(&envelope.Data)
	if profile.UserID == 0 {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, "business bridge returned empty user_id", nil)
	}
	return profile, nil
}

func (s *authService) allocateState(ctx context.Context, tenantKey string, provider string, clientType string) (string, error) {
	for range maxStateStoreRetry {
		state := strings.ReplaceAll(uuid.NewString(), "-", "")
		ok, err := s.redisRepo.SetIfAbsent(ctx, s.stateKey(tenantKey, provider, clientType, state), "1", time.Duration(s.authConfig.StateTTLSecond)*time.Second)
		if err != nil {
			return "", appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
		}
		if ok {
			return state, nil
		}
	}
	return "", appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, "failed to allocate login state", nil)
}

func (s *authService) stateKey(tenantKey string, provider string, clientType string, state string) string {
	return fmt.Sprintf("auth:state:%s:%s:%s:%s", normalize(tenantKey), normalize(provider), normalize(clientType), strings.TrimSpace(state))
}

func (s *authService) phoneCaptchaPrefix(tenantKey string, clientType string) string {
	return fmt.Sprintf("auth:phone:%s:%s", normalize(tenantKey), normalize(clientType))
}

func (s *authService) guestDevicePrefix(tenantKey string, clientType string) string {
	return fmt.Sprintf("auth:guest:%s:%s", normalize(tenantKey), normalize(clientType))
}

func buildUserResponse(user *model.AuthUser, tenant *model.AuthTenant) dto.AuthUserResponse {
	return dto.AuthUserResponse{
		ID:          user.ID,
		TenantKey:   tenant.TenantKey,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
		Status:      user.Status,
		LastLoginAt: user.LastLoginAt,
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func defaultDisplayName(provider string, subject string) string {
	suffix := subject
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	switch normalize(provider) {
	case "wechat":
		return "微信用户_" + suffix
	case "apple":
		return "Apple用户_" + suffix
	case "phone":
		return "手机用户_" + suffix
	case "guest":
		return "游客_" + suffix
	default:
		return "用户_" + suffix
	}
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 7 {
		return "手机用户"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func marshalJSON(value interface{}) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(payload)
}

func parseSessionMetadata(raw string) (*sessionMetadata, error) {
	metadata := &sessionMetadata{}
	if strings.TrimSpace(raw) == "" {
		return metadata, nil
	}
	if err := json.Unmarshal([]byte(raw), metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

func normalizeBusinessProfile(profile *businessUserProfile) *businessUserProfile {
	if profile == nil {
		return &businessUserProfile{}
	}
	normalized := *profile
	normalized.DisplayName = firstNonEmpty(profile.DisplayName, fmt.Sprintf("用户_%d", profile.UserID))
	normalized.AvatarURL = strings.TrimSpace(profile.AvatarURL)
	normalized.Role = firstNonEmpty(profile.Role, "user")
	normalized.Status = firstNonEmpty(profile.Status, "active")
	return &normalized
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type phoneCaptchaStoreAdapter struct {
	repo   redisRepo.KVRepository
	prefix string
}

func (a phoneCaptchaStoreAdapter) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return a.repo.Set(ctx, a.buildKey(key), value, expiration)
}

func (a phoneCaptchaStoreAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.repo.Get(ctx, a.buildKey(key))
}

func (a phoneCaptchaStoreAdapter) Delete(ctx context.Context, key string) error {
	return a.repo.Delete(ctx, a.buildKey(key))
}

func (a phoneCaptchaStoreAdapter) buildKey(key string) string {
	return a.prefix + ":" + strings.TrimSpace(key)
}

type guestDeviceStoreAdapter struct {
	repo   redisRepo.KVRepository
	prefix string
}

func (a guestDeviceStoreAdapter) Set(ctx context.Context, deviceID string, expiration time.Duration) error {
	return a.repo.Set(ctx, a.buildKey(deviceID), "1", expiration)
}

func (a guestDeviceStoreAdapter) Exists(ctx context.Context, deviceID string) (bool, error) {
	return a.repo.Exists(ctx, a.buildKey(deviceID))
}

func (a guestDeviceStoreAdapter) buildKey(key string) string {
	return a.prefix + ":" + strings.TrimSpace(key)
}
