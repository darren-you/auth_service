package logic

import (
	"fmt"
	"strings"

	"github.com/darren-you/auth_service/providerkeys"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
)

func (s *authFlow) loginWithWeChatApp(req *ProviderCallbackRequest) (*SessionResponse, error) {
	if err := ensureWeChatProviderClientType(providerkeys.ProviderWeChatApp, req.ClientType); err != nil {
		return nil, err
	}

	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, providerkeys.ProviderWeChatApp, req.ClientType)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(req.Code)
	state := strings.TrimSpace(req.State)
	if code == "" || state == "" {
		return nil, appErrors.ErrBadRequest
	}
	if err := s.consumeProviderState(tenant, providerConfig, state); err != nil {
		return nil, err
	}

	client := newWeChatAppProviderClient(providerConfig)
	oauthToken, err := client.ExchangeCode(s.ctx, code)
	if err != nil {
		s.Errorf(
			"wechat app login exchange code failed: tenant=%s code=%s state=%s err=%v",
			tenant.TenantKey,
			maskTail(code, 6),
			maskTail(state, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("exchange wechat app code failed: %w", err),
		)
	}
	oauthToken, err = client.EnsureAccessTokenValid(s.ctx, oauthToken)
	if err != nil {
		s.Errorf(
			"wechat app login verify access token failed: tenant=%s openid=%s err=%v",
			tenant.TenantKey,
			maskTail(oauthToken.OpenID, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("verify wechat app access token failed: %w", err),
		)
	}

	userInfo, err := client.FetchUserInfo(s.ctx, oauthToken.AccessToken, oauthToken.OpenID)
	if err != nil {
		s.Errorf("fetch wechat app user info failed: %v", err)
	}

	displayName := defaultDisplayName(providerkeys.ProviderWeChatApp, oauthToken.OpenID)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if userInfo != nil {
		if strings.TrimSpace(userInfo.Nickname) != "" {
			displayName = strings.TrimSpace(userInfo.Nickname)
		}
		if strings.TrimSpace(userInfo.HeadImgURL) != "" {
			avatarURL = strings.TrimSpace(userInfo.HeadImgURL)
		}
	}

	user, err := s.upsertIdentityUser(
		tenant,
		providerConfig,
		providerkeys.ProviderWeChatApp,
		oauthToken.OpenID,
		oauthToken.UnionID,
		displayName,
		avatarURL,
		"user",
		marshalJSON(userInfo),
	)
	if err != nil {
		s.Errorf(
			"wechat app login upsert identity user failed: tenant=%s openid=%s err=%v",
			tenant.TenantKey,
			maskTail(oauthToken.OpenID, 6),
			err,
		)
		return nil, err
	}

	businessUser, err := s.syncWeChatBusinessUser(tenant, providerConfig, oauthToken.OpenID, oauthToken.UnionID, "", displayName, avatarURL, req)
	if err != nil {
		s.Errorf(
			"wechat app login sync business user failed: tenant=%s openid=%s auth_user_id=%d err=%v",
			tenant.TenantKey,
			maskTail(oauthToken.OpenID, 6),
			user.ID,
			err,
		)
		return nil, err
	}

	return s.issueSession(tenant, user.ID, providerkeys.ProviderWeChatApp, providerConfig.ClientType, businessUser)
}
