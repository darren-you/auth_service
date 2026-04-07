package logic

import (
	"fmt"
	"strings"

	"github.com/darren-you/auth_service/providerkeys"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
)

func (s *authFlow) loginWithWeChatMiniProgram(req *ProviderCallbackRequest) (*SessionResponse, error) {
	if err := ensureWeChatProviderClientType(providerkeys.ProviderWeChatMiniProgram, req.ClientType); err != nil {
		return nil, err
	}

	tenant, providerConfig, err := s.resolveTenantAndProvider(req.TenantKey, providerkeys.ProviderWeChatMiniProgram, req.ClientType)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, appErrors.ErrBadRequest
	}

	client := newWeChatMiniProgramProviderClient(providerConfig)
	sessionResp, err := client.ExchangeCode(s.ctx, code)
	if err != nil {
		s.Errorf(
			"wechat miniprogram login exchange code failed: tenant=%s code=%s err=%v",
			tenant.TenantKey,
			maskTail(code, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("exchange wechat miniprogram code failed: %w", err),
		)
	}

	displayName := firstNonEmpty(req.DisplayName, defaultDisplayName(providerkeys.ProviderWeChatMiniProgram, sessionResp.OpenID))
	avatarURL := strings.TrimSpace(req.AvatarURL)
	user, err := s.upsertIdentityUser(
		tenant,
		providerConfig,
		providerkeys.ProviderWeChatMiniProgram,
		sessionResp.OpenID,
		sessionResp.UnionID,
		displayName,
		avatarURL,
		"user",
		marshalJSON(sessionResp),
	)
	if err != nil {
		s.Errorf(
			"wechat miniprogram login upsert identity user failed: tenant=%s openid=%s err=%v",
			tenant.TenantKey,
			maskTail(sessionResp.OpenID, 6),
			err,
		)
		return nil, err
	}

	businessUser, err := s.syncWeChatBusinessUser(tenant, providerConfig, sessionResp.OpenID, sessionResp.UnionID, sessionResp.SessionKey, displayName, avatarURL, req)
	if err != nil {
		s.Errorf(
			"wechat miniprogram login sync business user failed: tenant=%s openid=%s auth_user_id=%d err=%v",
			tenant.TenantKey,
			maskTail(sessionResp.OpenID, 6),
			user.ID,
			err,
		)
		return nil, err
	}

	return s.issueSession(tenant, user.ID, providerkeys.ProviderWeChatMiniProgram, providerConfig.ClientType, businessUser)
}
