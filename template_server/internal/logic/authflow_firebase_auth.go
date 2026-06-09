package logic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/darren-you/auth_service/providerkeys"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/model"
	firebaseauth "github.com/darren-you/auth_service/template_server/pkg/provider/firebase_auth"
)

type firebaseProviderExtraConfig struct {
	ProjectID            string `json:"project_id"`
	JWKSURL              string `json:"jwks_url"`
	RequestTimeoutSecond int    `json:"request_timeout_second"`
}

func (s *authFlow) loginWithFirebaseAuth(req *ProviderCallbackRequest) (*SessionResponse, error) {
	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, providerkeys.ProviderFirebaseAuth, req.ClientType)
	if err != nil {
		return nil, err
	}

	idToken := strings.TrimSpace(req.Token)
	if idToken == "" {
		return nil, appErrors.ErrBadRequest
	}

	client, err := newFirebaseAuthProviderClient(providerConfig)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, appErrors.ErrConfigInvalid.Message, err)
	}
	verifiedToken, err := client.VerifyIDToken(s.ctx, idToken)
	if err != nil {
		s.Errorf("firebase auth verify id token failed: tenant=%s provider=%s err=%v", tenant.TenantKey, providerConfig.Provider, err)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("verify firebase id token failed: %w", err),
		)
	}

	displayName := firstNonEmpty(verifiedToken.DisplayName, req.DisplayName, defaultDisplayName(providerkeys.ProviderFirebaseAuth, verifiedToken.UID))
	avatarURL := firstNonEmpty(verifiedToken.AvatarURL, req.AvatarURL)
	profileJSON := marshalJSON(verifiedToken.RawClaims)
	user, err := s.upsertIdentityUser(
		tenant,
		providerConfig,
		providerkeys.ProviderFirebaseAuth,
		verifiedToken.UID,
		"",
		displayName,
		avatarURL,
		"user",
		profileJSON,
	)
	if err != nil {
		s.Errorf(
			"firebase auth upsert identity user failed: tenant=%s uid=%s err=%v",
			tenant.TenantKey,
			maskTail(verifiedToken.UID, 6),
			err,
		)
		return nil, err
	}

	businessUser, err := s.syncBusinessUser(tenant.TenantKey, "firebase", providerConfig.ClientType, businessBridgeRequest{
		ProviderSubject: verifiedToken.UID,
		Email:           verifiedToken.Email,
		EmailVerified:   verifiedToken.EmailVerified,
		DisplayName:     displayName,
		AvatarURL:       avatarURL,
		CurrentUserID:   uint(req.CurrentUserID),
		CurrentUserRole: req.CurrentUserRole,
	})
	if err != nil {
		s.Errorf(
			"firebase auth sync business user failed: tenant=%s uid=%s auth_user_id=%d err=%v",
			tenant.TenantKey,
			maskTail(verifiedToken.UID, 6),
			user.ID,
			err,
		)
		return nil, err
	}
	user, err = s.bindProviderIdentityToBusinessUser(
		tenant,
		providerConfig,
		providerkeys.ProviderFirebaseAuth,
		verifiedToken.UID,
		"",
		displayName,
		avatarURL,
		profileJSON,
		user,
		businessUser.UserID,
	)
	if err != nil {
		s.Errorf(
			"firebase auth bind identity to business user failed: tenant=%s uid=%s auth_user_id=%d business_user_id=%d err=%v",
			tenant.TenantKey,
			maskTail(verifiedToken.UID, 6),
			authUserIDForLog(user),
			businessUser.UserID,
			err,
		)
		return nil, err
	}

	return s.issueSession(tenant, user.ID, providerkeys.ProviderFirebaseAuth, providerConfig.ClientType, businessUser)
}

func newFirebaseAuthProviderClient(providerConfig *model.AuthProviderConfig) (*firebaseauth.Client, error) {
	extraCfg := parseFirebaseProviderExtraConfig(providerConfig.ExtraJSON)
	return firebaseauth.NewClient(firebaseauth.Config{
		ProjectID:            firstNonEmpty(extraCfg.ProjectID, providerConfig.ClientID),
		JWKSURL:              extraCfg.JWKSURL,
		RequestTimeoutSecond: extraCfg.RequestTimeoutSecond,
	})
}

func parseFirebaseProviderExtraConfig(raw string) firebaseProviderExtraConfig {
	var cfg firebaseProviderExtraConfig
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}
