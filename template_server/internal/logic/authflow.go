package logic

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

	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
	authguest "github.com/darren-you/auth_service/template_server/pkg/guest"
	authphone "github.com/darren-you/auth_service/template_server/pkg/phone"
	authapple "github.com/darren-you/auth_service/template_server/pkg/provider/apple"
	authgetui "github.com/darren-you/auth_service/template_server/pkg/provider/getui"
	authtencentsms "github.com/darren-you/auth_service/template_server/pkg/provider/tencentsms"
	authwechat "github.com/darren-you/auth_service/template_server/pkg/provider/wechat"
	"github.com/darren-you/auth_service/template_server/pkg/session"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

const maxStateStoreRetry = 3

type (
	ProviderCallbackRequest  = types.ProviderCallbackReq
	PhoneCaptchaSendRequest  = types.PhoneCaptchaSendReq
	GuestDeviceIDRequest     = types.GuestDeviceIDReq
	PasswordRegisterRequest  = types.PasswordRegisterReq
	RefreshTokenRequest      = types.RefreshReq
	LogoutRequest            = types.LogoutReq
	LoginURLResponse         = types.LoginURLResp
	PhoneCaptchaSendResponse = types.PhoneCaptchaSendResp
	GuestDeviceIDResponse    = types.GuestDeviceIDResp
	AuthUserResponse         = types.AuthUserResp
	SessionResponse          = types.SessionResp
)

type authFlow struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
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

func newAuthFlow(ctx context.Context, svcCtx *svc.ServiceContext) *authFlow {
	return &authFlow{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (s *authFlow) GetLoginURL(req *types.GetLoginURLReq) (*LoginURLResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, req.Provider, req.ClientType)
	if err != nil {
		return nil, err
	}

	if req.Provider != "wechat" {
		return nil, appErrors.ErrUnsupportedProvider
	}

	state, err := s.allocateState(tenant.TenantKey, req.Provider, providerConfig.ClientType)
	if err != nil {
		return nil, err
	}

	if providerConfig.ClientType == "app" && strings.TrimSpace(providerConfig.RedirectURI) == "" {
		return &LoginURLResponse{
			TenantKey:  tenant.TenantKey,
			Provider:   req.Provider,
			ClientType: providerConfig.ClientType,
			LoginURL:   "",
			State:      state,
		}, nil
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

	return &LoginURLResponse{
		TenantKey:  tenant.TenantKey,
		Provider:   req.Provider,
		ClientType: providerConfig.ClientType,
		LoginURL:   loginURL,
		State:      state,
	}, nil
}

func (s *authFlow) ProviderCallback(req *ProviderCallbackRequest) (*SessionResponse, error) {
	switch normalize(req.Provider) {
	case "wechat":
		return s.loginWithWeChat(req)
	case "apple":
		return s.loginWithApple(req)
	case "password":
		return s.loginWithPassword(req)
	case "phone":
		return s.loginWithPhone(req)
	case "guest":
		return s.loginWithGuest(req)
	default:
		return nil, appErrors.ErrUnsupportedProvider
	}
}

func (s *authFlow) RegisterPassword(req *PasswordRegisterRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "password", req.ClientType)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	if username == "" || email == "" || password == "" {
		return nil, appErrors.ErrBadRequest
	}

	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "password", providerConfig.ClientType, businessBridgeRequest{
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

	return s.issuePasswordSession(tenant, providerConfig, businessUser, username, req.AvatarURL)
}

func (s *authFlow) IssueGuestDeviceID(req *GuestDeviceIDRequest) (*GuestDeviceIDResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "guest", req.ClientType)
	if err != nil {
		return nil, err
	}

	guestService := authguest.NewService(
		guestDeviceStoreAdapter{
			store:  s.svcCtx.KVStore,
			prefix: s.guestDevicePrefix(tenant.TenantKey, providerConfig.ClientType),
		},
		time.Duration(s.svcCtx.Config.Auth.GuestDeviceTTLSecond)*time.Second,
	)

	result, err := guestService.IssueVirtualDeviceID(s.ctx)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	return &GuestDeviceIDResponse{
		TenantKey:  tenant.TenantKey,
		ClientType: providerConfig.ClientType,
		DeviceID:   result.DeviceID,
		ExpiresIn:  int64(result.ExpiresIn),
	}, nil
}

func (s *authFlow) Refresh(req *RefreshTokenRequest) (*SessionResponse, error) {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return nil, appErrors.ErrBadRequest
	}

	claims, err := session.ParseRefreshToken(refreshToken, s.svcCtx.SessionConfig)
	if err != nil {
		if err == session.ErrExpiredToken {
			return nil, appErrors.ErrTokenExpired
		}
		return nil, appErrors.ErrTokenInvalid
	}

	sessionRecord, err := s.svcCtx.AuthRepo.FindSessionByHash(s.ctx, hashToken(refreshToken))
	if err != nil {
		s.Errorf("refresh find session failed: hash=%s err=%v", hashToken(refreshToken), err)
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
		s.Errorf("refresh parse session metadata failed: auth_user_id=%d err=%v", sessionRecord.AuthUserID, err)
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if metadata.TokenUserID == 0 || metadata.TokenUserID != claims.UserID {
		return nil, appErrors.ErrSessionRevoked
	}

	tenant, err := s.svcCtx.AuthRepo.FindTenantByID(s.ctx, sessionRecord.TenantID)
	if err != nil {
		s.Errorf("refresh find tenant failed: tenant_id=%d err=%v", sessionRecord.TenantID, err)
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil {
		return nil, appErrors.ErrTenantNotFound
	}

	if err := s.svcCtx.AuthRepo.RevokeSessionByHash(s.ctx, sessionRecord.RefreshTokenHash, time.Now()); err != nil {
		s.Errorf("refresh revoke session failed: auth_user_id=%d hash=%s err=%v", sessionRecord.AuthUserID, sessionRecord.RefreshTokenHash, err)
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	resp, err := s.issueSession(tenant, sessionRecord.AuthUserID, sessionRecord.Provider, sessionRecord.ClientType, &businessUserProfile{
		UserID:      metadata.TokenUserID,
		DisplayName: metadata.DisplayName,
		AvatarURL:   metadata.AvatarURL,
		Role:        metadata.Role,
		Status:      metadata.Status,
	})
	if err != nil {
		s.Errorf("refresh issue session failed: tenant_id=%d auth_user_id=%d business_user_id=%d err=%v", sessionRecord.TenantID, sessionRecord.AuthUserID, metadata.TokenUserID, err)
		return nil, err
	}
	return resp, nil
}

func (s *authFlow) Logout(req *LogoutRequest) error {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return appErrors.ErrBadRequest
	}

	if _, err := session.ParseRefreshTokenIgnoringExpiry(refreshToken, s.svcCtx.SessionConfig); err != nil {
		return appErrors.ErrTokenInvalid
	}

	if err := s.svcCtx.AuthRepo.RevokeSessionByHash(s.ctx, hashToken(refreshToken), time.Now()); err != nil {
		return appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	return nil
}

func (s *authFlow) GetUserProfileByID(userID uint) (*AuthUserResponse, error) {
	user, err := s.svcCtx.AuthRepo.FindUserByID(s.ctx, userID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if user == nil {
		return nil, appErrors.ErrUnauthorized
	}

	tenant, err := s.svcCtx.AuthRepo.FindTenantByID(s.ctx, user.TenantID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil {
		return nil, appErrors.ErrTenantNotFound
	}

	profile := buildUserResponse(user, tenant)
	return &profile, nil
}

func (s *authFlow) loginWithWeChat(req *ProviderCallbackRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "wechat", req.ClientType)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(req.Code)
	state := strings.TrimSpace(req.State)
	if code == "" || state == "" {
		return nil, appErrors.ErrBadRequest
	}

	ok, err := s.svcCtx.KVStore.Consume(s.ctx, s.stateKey(tenant.TenantKey, "wechat", providerConfig.ClientType, state))
	if err != nil {
		s.Errorf(
			"wechat login consume state failed: tenant=%s client_type=%s state=%s err=%v",
			tenant.TenantKey,
			providerConfig.ClientType,
			maskTail(state, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("consume wechat login state failed: %w", err),
		)
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
	oauthToken, err := client.ExchangeCode(s.ctx, code)
	if err != nil {
		s.Errorf(
			"wechat login exchange code failed: tenant=%s client_type=%s code=%s state=%s err=%v",
			tenant.TenantKey,
			providerConfig.ClientType,
			maskTail(code, 6),
			maskTail(state, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("exchange wechat code failed: %w", err),
		)
	}
	oauthToken, err = client.EnsureAccessTokenValid(s.ctx, oauthToken)
	if err != nil {
		s.Errorf(
			"wechat login verify access token failed: tenant=%s client_type=%s openid=%s err=%v",
			tenant.TenantKey,
			providerConfig.ClientType,
			maskTail(oauthToken.OpenID, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("verify wechat access token failed: %w", err),
		)
	}

	userInfo, err := client.FetchUserInfo(s.ctx, oauthToken.AccessToken, oauthToken.OpenID)
	if err != nil {
		s.Errorf("fetch wechat user info failed: %v", err)
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

	user, err := s.upsertIdentityUser(tenant, providerConfig, "wechat", oauthToken.OpenID, oauthToken.UnionID, displayName, avatarURL, "user", marshalJSON(userInfo))
	if err != nil {
		s.Errorf(
			"wechat login upsert identity user failed: tenant=%s client_type=%s openid=%s err=%v",
			tenant.TenantKey,
			providerConfig.ClientType,
			maskTail(oauthToken.OpenID, 6),
			err,
		)
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "wechat", providerConfig.ClientType, businessBridgeRequest{
		OpenID:          oauthToken.OpenID,
		UnionID:         oauthToken.UnionID,
		DisplayName:     displayName,
		AvatarURL:       avatarURL,
		CurrentUserID:   uint(req.CurrentUserID),
		CurrentUserRole: req.CurrentUserRole,
	})
	if err != nil {
		s.Errorf(
			"wechat login sync business user failed: tenant=%s client_type=%s openid=%s auth_user_id=%d err=%v",
			tenant.TenantKey,
			providerConfig.ClientType,
			maskTail(oauthToken.OpenID, 6),
			user.ID,
			err,
		)
		return nil, err
	}
	return s.issueSession(tenant, user.ID, "wechat", providerConfig.ClientType, businessUser)
}

func (s *authFlow) loginWithApple(req *ProviderCallbackRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "apple", req.ClientType)
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
	validationResp, err := client.VerifyAuthorizationCode(s.ctx, authCode)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	uniqueID, err := client.GetUniqueIDFromIDToken(validationResp.IDToken)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	displayName := firstNonEmpty(req.DisplayName, defaultDisplayName("apple", uniqueID))
	user, err := s.upsertIdentityUser(tenant, providerConfig, "apple", uniqueID, "", displayName, req.AvatarURL, "user", marshalJSON(validationResp))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "apple", providerConfig.ClientType, businessBridgeRequest{
		AppleUserID:     uniqueID,
		DisplayName:     displayName,
		AvatarURL:       req.AvatarURL,
		CurrentUserID:   uint(req.CurrentUserID),
		CurrentUserRole: req.CurrentUserRole,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(tenant, user.ID, "apple", providerConfig.ClientType, businessUser)
}

func (s *authFlow) loginWithPassword(req *ProviderCallbackRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "password", req.ClientType)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return nil, appErrors.ErrBadRequest
	}

	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "password", providerConfig.ClientType, businessBridgeRequest{
		Action:      "login",
		Username:    username,
		Password:    password,
		DisplayName: firstNonEmpty(req.DisplayName, username),
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return nil, err
	}

	return s.issuePasswordSession(tenant, providerConfig, businessUser, username, req.AvatarURL)
}

func (s *authFlow) loginWithPhone(req *ProviderCallbackRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "phone", req.ClientType)
	if err != nil {
		return nil, err
	}

	hasCaptchaMode := strings.TrimSpace(req.Phone) != "" || strings.TrimSpace(req.Captcha) != "" || strings.TrimSpace(req.CaptchaKey) != ""
	hasOneClickMode := strings.TrimSpace(req.Token) != "" || strings.TrimSpace(req.Gyuid) != ""
	if hasCaptchaMode == hasOneClickMode {
		return nil, appErrors.ErrBadRequest
	}

	if hasCaptchaMode {
		return s.loginWithPhoneCaptcha(tenant, providerConfig, req)
	}
	return s.loginWithPhoneOneClick(tenant, providerConfig, req)
}

func (s *authFlow) loginWithGuest(req *ProviderCallbackRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "guest", req.ClientType)
	if err != nil {
		return nil, err
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, appErrors.ErrBadRequest
	}

	guestService := authguest.NewService(
		guestDeviceStoreAdapter{
			store:  s.svcCtx.KVStore,
			prefix: s.guestDevicePrefix(tenant.TenantKey, providerConfig.ClientType),
		},
		time.Duration(s.svcCtx.Config.Auth.GuestDeviceTTLSecond)*time.Second,
	)
	valid, err := guestService.IsValid(s.ctx, deviceID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if !valid {
		return nil, appErrors.ErrUnauthorized
	}

	displayName := firstNonEmpty(req.DisplayName, authguest.UsernameFromDeviceID(deviceID))
	user, err := s.upsertIdentityUser(tenant, providerConfig, "guest", deviceID, "", displayName, req.AvatarURL, "guest", marshalJSON(map[string]string{"device_id": deviceID}))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "guest", providerConfig.ClientType, businessBridgeRequest{
		DeviceID:    deviceID,
		DisplayName: displayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(tenant, user.ID, "guest", providerConfig.ClientType, businessUser)
}

func (s *authFlow) issuePasswordSession(
	tenant *model.AuthTenant,
	providerConfig *model.AuthProviderConfig,
	businessUser *businessUserProfile,
	username string,
	avatarURL string,
) (*SessionResponse, error) {
	if businessUser == nil || businessUser.UserID == 0 {
		return nil, appErrors.ErrAuthFailed
	}

	subject := strconv.FormatUint(uint64(businessUser.UserID), 10)
	displayName := firstNonEmpty(businessUser.DisplayName, username)
	normalizedAvatarURL := firstNonEmpty(businessUser.AvatarURL, avatarURL)
	user, err := s.upsertIdentityUser(
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
	return s.issueSession(tenant, user.ID, "password", providerConfig.ClientType, businessUser)
}

func (s *authFlow) resolveTenantAndProvider(tenantKey string, provider string, clientType string) (*model.AuthTenant, *model.AuthProviderConfig, error) {
	tenant, err := s.svcCtx.AuthRepo.FindTenantByKey(s.ctx, normalize(tenantKey))
	if err != nil {
		return nil, nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil || !tenant.Enabled {
		return nil, nil, appErrors.ErrTenantNotFound
	}

	providerConfig, err := s.svcCtx.AuthRepo.FindProviderConfig(s.ctx, tenant.ID, normalize(provider), normalize(clientType))
	if err != nil {
		return nil, nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if providerConfig == nil || !providerConfig.Enabled {
		return nil, nil, appErrors.ErrProviderNotEnabled
	}

	return tenant, providerConfig, nil
}

func (s *authFlow) upsertIdentityUser(
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
	user, identity, err := s.svcCtx.AuthRepo.FindUserByIdentity(s.ctx, tenant.ID, provider, subject)
	if err != nil {
		return nil, appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("find auth user by identity failed: %w", err),
		)
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
		if err := s.svcCtx.AuthRepo.CreateUserWithIdentity(s.ctx, user, identity); err != nil {
			return nil, appErrors.New(
				appErrors.ErrInternalServer.Code,
				appErrors.ErrInternalServer.HTTPStatus,
				appErrors.ErrInternalServer.Message,
				fmt.Errorf("create auth user with identity failed: %w", err),
			)
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
	if err := s.svcCtx.AuthRepo.UpdateUserLogin(s.ctx, user.ID, updatedDisplayName, updatedAvatarURL, now); err != nil {
		return nil, appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("update auth user login failed: %w", err),
		)
	}
	if identity != nil {
		if err := s.svcCtx.AuthRepo.UpdateIdentity(s.ctx, identity.ID, providerConfig.ClientType, strings.TrimSpace(unionID), profileJSON); err != nil {
			return nil, appErrors.New(
				appErrors.ErrInternalServer.Code,
				appErrors.ErrInternalServer.HTTPStatus,
				appErrors.ErrInternalServer.Message,
				fmt.Errorf("update auth identity failed: %w", err),
			)
		}
	}
	user.DisplayName = updatedDisplayName
	user.AvatarURL = updatedAvatarURL
	user.LastLoginAt = &now
	return user, nil
}

func (s *authFlow) issueSession(tenant *model.AuthTenant, authUserID uint, provider string, clientType string, businessUser *businessUserProfile) (*SessionResponse, error) {
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
		s.svcCtx.SessionConfig,
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
		ExpiresAt:        now.Add(s.svcCtx.SessionConfig.RefreshExpiry),
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
	if err := s.svcCtx.AuthRepo.CreateSession(s.ctx, authSession); err != nil {
		return nil, appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("create auth session failed: %w", err),
		)
	}

	return &SessionResponse{
		TenantKey:    tenant.TenantKey,
		Provider:     provider,
		ClientType:   clientType,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		User: AuthUserResponse{
			Id:          uint64(normalizedProfile.UserID),
			TenantKey:   tenant.TenantKey,
			DisplayName: normalizedProfile.DisplayName,
			AvatarURL:   normalizedProfile.AvatarURL,
			Role:        normalizedProfile.Role,
			Status:      normalizedProfile.Status,
		},
	}, nil
}

func (s *authFlow) resolveTenantRuntimeConfig(tenantKey string) (*tenantRuntimeConfig, error) {
	normalizedTenantKey := normalize(tenantKey)
	for _, tenant := range s.svcCtx.Config.Auth.Tenants {
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

func (s *authFlow) syncBusinessUser(tenantKey string, provider string, clientType string, req businessBridgeRequest) (*businessUserProfile, error) {
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
		return nil, appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("marshal business bridge request failed: %w", err),
		)
	}

	httpReq, err := http.NewRequestWithContext(s.ctx, http.MethodPost, runtimeCfg.BridgeBaseURL+"/api/v1/internal/auth/sync", bytes.NewReader(payload))
	if err != nil {
		return nil, appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("build business bridge request failed: %w", err),
		)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Auth-Service-Key", runtimeCfg.BridgeAuthKey)

	httpResp, err := (&http.Client{Timeout: 8 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("request business bridge failed: %w", err),
		)
	}
	defer httpResp.Body.Close()

	var envelope bridgeEnvelope
	if err := json.NewDecoder(httpResp.Body).Decode(&envelope); err != nil {
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("decode business bridge response failed: %w", err),
		)
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

func (s *authFlow) allocateState(tenantKey string, provider string, clientType string) (string, error) {
	for range maxStateStoreRetry {
		state := strings.ReplaceAll(uuid.NewString(), "-", "")
		ok, err := s.svcCtx.KVStore.SetIfAbsent(s.ctx, s.stateKey(tenantKey, provider, clientType, state), "1", time.Duration(s.svcCtx.Config.Auth.StateTTLSecond)*time.Second)
		if err != nil {
			return "", appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
		}
		if ok {
			return state, nil
		}
	}
	return "", appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, "failed to allocate login state", nil)
}

func (s *authFlow) stateKey(tenantKey string, provider string, clientType string, state string) string {
	return fmt.Sprintf("auth:state:%s:%s:%s:%s", normalize(tenantKey), normalize(provider), normalize(clientType), strings.TrimSpace(state))
}

func (s *authFlow) phoneCaptchaPrefix(tenantKey string, clientType string) string {
	return fmt.Sprintf("auth:phone:%s:%s", normalize(tenantKey), normalize(clientType))
}

func (s *authFlow) guestDevicePrefix(tenantKey string, clientType string) string {
	return fmt.Sprintf("auth:guest:%s:%s", normalize(tenantKey), normalize(clientType))
}

func buildUserResponse(user *model.AuthUser, tenant *model.AuthTenant) AuthUserResponse {
	var lastLoginAt int64
	if user.LastLoginAt != nil {
		lastLoginAt = user.LastLoginAt.UnixMilli()
	}

	return AuthUserResponse{
		Id:          uint64(user.ID),
		TenantKey:   tenant.TenantKey,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
		Status:      user.Status,
		LastLoginAt: lastLoginAt,
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

func marshalJSON(value any) string {
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

func maskTail(value string, keep int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if keep <= 0 || len(trimmed) <= keep {
		return trimmed
	}
	return "..." + trimmed[len(trimmed)-keep:]
}

func (s *authFlow) SendPhoneCaptcha(req *PhoneCaptchaSendRequest) (*PhoneCaptchaSendResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, "phone", req.ClientType)
	if err != nil {
		return nil, err
	}

	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		return nil, appErrors.ErrBadRequest
	}

	var sender authphone.Sender
	isTestPhone := providerConfig.TestPhone != "" && phone == providerConfig.TestPhone
	if !isTestPhone {
		extra, err := parsePhoneProviderExtra(providerConfig.ExtraJSON)
		if err != nil {
			return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "phone provider extra_json is invalid", err)
		}
		if extra.SMS == nil || !extra.SMS.IsConfigured() {
			return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "phone sender is not configured", nil)
		}
		sender = authtencentsms.NewSender(authtencentsms.Config{
			SecretID:    extra.SMS.SecretID,
			SecretKey:   extra.SMS.SecretKey,
			SmsSDKAppID: extra.SMS.SmsSDKAppID,
			SignName:    extra.SMS.SignName,
			TemplateID:  extra.SMS.TemplateID,
			Region:      extra.SMS.Region,
		})
	}

	phoneService := s.newPhoneService(tenant.TenantKey, providerConfig, sender)
	result, err := phoneService.Send(s.ctx, phone)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	return &PhoneCaptchaSendResponse{
		TenantKey:  tenant.TenantKey,
		ClientType: providerConfig.ClientType,
		CaptchaKey: result.CaptchaKey,
		ExpiresIn:  int64(result.ExpiresIn),
	}, nil
}

func (s *authFlow) loginWithPhoneCaptcha(tenant *model.AuthTenant, providerConfig *model.AuthProviderConfig, req *ProviderCallbackRequest) (*SessionResponse, error) {
	phone := strings.TrimSpace(req.Phone)
	captcha := strings.TrimSpace(req.Captcha)
	captchaKey := strings.TrimSpace(req.CaptchaKey)
	if phone == "" || captcha == "" || captchaKey == "" {
		return nil, appErrors.ErrBadRequest
	}

	phoneService := s.newPhoneService(tenant.TenantKey, providerConfig, nil)
	if err := phoneService.Verify(s.ctx, authphone.VerifyRequest{
		Phone:      phone,
		Captcha:    captcha,
		CaptchaKey: captchaKey,
	}); err != nil {
		return nil, appErrors.ErrCaptchaInvalid
	}

	return s.issuePhoneSession(tenant, providerConfig, phone, req.DisplayName, req.AvatarURL, map[string]string{
		"phone":        phone,
		"login_method": "captcha",
	})
}

func (s *authFlow) loginWithPhoneOneClick(tenant *model.AuthTenant, providerConfig *model.AuthProviderConfig, req *ProviderCallbackRequest) (*SessionResponse, error) {
	token := strings.TrimSpace(req.Token)
	gyuid := strings.TrimSpace(req.Gyuid)
	if token == "" || gyuid == "" {
		return nil, appErrors.ErrBadRequest
	}

	extra, err := parsePhoneProviderExtra(providerConfig.ExtraJSON)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "phone provider extra_json is invalid", err)
	}
	if extra.Getui == nil || !extra.Getui.IsConfigured() {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "getui quick login is not configured", nil)
	}

	client := authgetui.NewClient(authgetui.Config{
		AppID:        extra.Getui.AppID,
		AppKey:       extra.Getui.AppKey,
		AppSecret:    extra.Getui.AppSecret,
		MasterSecret: extra.Getui.MasterSecret,
		BaseURL:      extra.Getui.BaseURL,
	})
	phone, err := client.OneClickLogin(s.ctx, token, gyuid)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrAuthFailed.Code, appErrors.ErrAuthFailed.HTTPStatus, appErrors.ErrAuthFailed.Message, err)
	}

	return s.issuePhoneSession(tenant, providerConfig, phone, req.DisplayName, req.AvatarURL, map[string]string{
		"phone":        phone,
		"login_method": "one_click",
		"gyuid":        gyuid,
	})
}

func (s *authFlow) issuePhoneSession(tenant *model.AuthTenant, providerConfig *model.AuthProviderConfig, phone string, displayName string, avatarURL string, profile map[string]string) (*SessionResponse, error) {
	displayName = firstNonEmpty(displayName, maskPhone(phone))
	user, err := s.upsertIdentityUser(tenant, providerConfig, "phone", phone, "", displayName, avatarURL, "user", marshalJSON(profile))
	if err != nil {
		return nil, err
	}
	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "phone", providerConfig.ClientType, businessBridgeRequest{
		Phone:       phone,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	})
	if err != nil {
		return nil, err
	}
	return s.issueSession(tenant, user.ID, "phone", providerConfig.ClientType, businessUser)
}

func (s *authFlow) newPhoneService(tenantKey string, providerConfig *model.AuthProviderConfig, sender authphone.Sender) *authphone.Service {
	return authphone.NewService(
		phoneCaptchaStoreAdapter{
			store:  s.svcCtx.KVStore,
			prefix: s.phoneCaptchaPrefix(tenantKey, providerConfig.ClientType),
		},
		sender,
		authphone.Config{
			TestPhone:      providerConfig.TestPhone,
			TestCaptcha:    providerConfig.TestCaptcha,
			TestCaptchaKey: providerConfig.TestCaptchaKey,
			TTL:            time.Duration(s.svcCtx.Config.Auth.PhoneCaptchaTTLSecond) * time.Second,
			CaptchaLength:  4,
		},
	)
}

type phoneProviderExtra struct {
	SMS   *phoneSMSExtra   `json:"sms,omitempty"`
	Getui *phoneGetuiExtra `json:"getui,omitempty"`
}

type phoneSMSExtra struct {
	SecretID    string `json:"secret_id,omitempty"`
	SecretKey   string `json:"secret_key,omitempty"`
	SmsSDKAppID string `json:"sms_sdk_app_id,omitempty"`
	SignName    string `json:"sign_name,omitempty"`
	TemplateID  string `json:"template_id,omitempty"`
	Region      string `json:"region,omitempty"`
}

type phoneGetuiExtra struct {
	AppID        string `json:"app_id,omitempty"`
	AppKey       string `json:"app_key,omitempty"`
	AppSecret    string `json:"app_secret,omitempty"`
	MasterSecret string `json:"master_secret,omitempty"`
	BaseURL      string `json:"base_url,omitempty"`
}

func parsePhoneProviderExtra(raw string) (*phoneProviderExtra, error) {
	extra := &phoneProviderExtra{}
	if strings.TrimSpace(raw) == "" {
		return extra, nil
	}
	if err := json.Unmarshal([]byte(raw), extra); err != nil {
		return nil, err
	}
	if extra.SMS != nil {
		extra.SMS.SecretID = strings.TrimSpace(extra.SMS.SecretID)
		extra.SMS.SecretKey = strings.TrimSpace(extra.SMS.SecretKey)
		extra.SMS.SmsSDKAppID = strings.TrimSpace(extra.SMS.SmsSDKAppID)
		extra.SMS.SignName = strings.TrimSpace(extra.SMS.SignName)
		extra.SMS.TemplateID = strings.TrimSpace(extra.SMS.TemplateID)
		extra.SMS.Region = strings.TrimSpace(extra.SMS.Region)
	}
	if extra.Getui != nil {
		extra.Getui.AppID = strings.TrimSpace(extra.Getui.AppID)
		extra.Getui.AppKey = strings.TrimSpace(extra.Getui.AppKey)
		extra.Getui.AppSecret = strings.TrimSpace(extra.Getui.AppSecret)
		extra.Getui.MasterSecret = strings.TrimSpace(extra.Getui.MasterSecret)
		extra.Getui.BaseURL = strings.TrimSpace(extra.Getui.BaseURL)
	}
	return extra, nil
}

func (c *phoneSMSExtra) IsConfigured() bool {
	return c != nil &&
		c.SecretID != "" &&
		c.SecretKey != "" &&
		c.SmsSDKAppID != "" &&
		c.SignName != "" &&
		c.TemplateID != ""
}

func (c *phoneGetuiExtra) IsConfigured() bool {
	return c != nil &&
		c.AppID != "" &&
		c.AppSecret != "" &&
		c.MasterSecret != ""
}

type phoneCaptchaStoreAdapter struct {
	store interface {
		Set(ctx context.Context, key string, value string, expiration time.Duration) error
		Get(ctx context.Context, key string) (string, error)
		Delete(ctx context.Context, key string) error
	}
	prefix string
}

func (a phoneCaptchaStoreAdapter) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return a.store.Set(ctx, a.buildKey(key), value, expiration)
}

func (a phoneCaptchaStoreAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.store.Get(ctx, a.buildKey(key))
}

func (a phoneCaptchaStoreAdapter) Delete(ctx context.Context, key string) error {
	return a.store.Delete(ctx, a.buildKey(key))
}

func (a phoneCaptchaStoreAdapter) buildKey(key string) string {
	return a.prefix + ":" + strings.TrimSpace(key)
}

type guestDeviceStoreAdapter struct {
	store interface {
		Set(ctx context.Context, key string, value string, expiration time.Duration) error
		Exists(ctx context.Context, key string) (bool, error)
	}
	prefix string
}

func (a guestDeviceStoreAdapter) Set(ctx context.Context, deviceID string, expiration time.Duration) error {
	return a.store.Set(ctx, a.buildKey(deviceID), "1", expiration)
}

func (a guestDeviceStoreAdapter) Exists(ctx context.Context, deviceID string) (bool, error) {
	return a.store.Exists(ctx, a.buildKey(deviceID))
}

func (a guestDeviceStoreAdapter) buildKey(key string) string {
	return a.prefix + ":" + strings.TrimSpace(key)
}
