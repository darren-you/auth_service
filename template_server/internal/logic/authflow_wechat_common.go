package logic

import (
	"fmt"

	"github.com/darren-you/auth_service/providerkeys"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/model"
	wechatapp "github.com/darren-you/auth_service/template_server/pkg/provider/wechat_app"
	wechatminiprogram "github.com/darren-you/auth_service/template_server/pkg/provider/wechat_miniprogram"
	wechatweb "github.com/darren-you/auth_service/template_server/pkg/provider/wechat_web"
)

func (s *authFlow) consumeProviderState(tenant *model.AuthTenant, providerConfig *model.AuthProviderConfig, state string) error {
	ok, err := s.svcCtx.KVStore.Consume(s.ctx, s.stateKey(tenant.TenantKey, providerConfig.Provider, providerConfig.ClientType, state))
	if err != nil {
		s.Errorf(
			"wechat provider consume state failed: provider=%s tenant=%s client_type=%s state=%s err=%v",
			providerConfig.Provider,
			tenant.TenantKey,
			providerConfig.ClientType,
			maskTail(state, 6),
			err,
		)
		return appErrors.New(
			appErrors.ErrInternalServer.Code,
			appErrors.ErrInternalServer.HTTPStatus,
			appErrors.ErrInternalServer.Message,
			fmt.Errorf("consume %s login state failed: %w", providerConfig.Provider, err),
		)
	}
	if !ok {
		return appErrors.ErrWeChatStateInvalid
	}
	return nil
}

func (s *authFlow) syncWeChatBusinessUser(
	tenant *model.AuthTenant,
	providerConfig *model.AuthProviderConfig,
	openID string,
	unionID string,
	displayName string,
	avatarURL string,
	req *ProviderCallbackRequest,
) (*businessUserProfile, error) {
	return s.syncBusinessUser(tenant.TenantKey, "wechat", providerConfig.ClientType, businessBridgeRequest{
		OpenID:          openID,
		UnionID:         unionID,
		DisplayName:     displayName,
		AvatarURL:       avatarURL,
		CurrentUserID:   uint(req.CurrentUserID),
		CurrentUserRole: req.CurrentUserRole,
	})
}

func ensureWeChatProviderClientType(provider string, clientType string) error {
	expectedClientType := providerkeys.WeChatClientType(provider)
	if expectedClientType == "" {
		return appErrors.ErrUnsupportedProvider
	}
	if providerkeys.NormalizeClientType(clientType) != expectedClientType {
		return appErrors.ErrBadRequest
	}
	return nil
}

func newWeChatAppProviderClient(providerConfig *model.AuthProviderConfig) *wechatapp.Client {
	return wechatapp.NewClient(wechatapp.Config{
		AppID:                providerConfig.AppID,
		AppSecret:            providerConfig.AppSecret,
		RequestTimeoutSecond: 0,
	})
}

func newWeChatWebProviderClient(providerConfig *model.AuthProviderConfig) *wechatweb.Client {
	return wechatweb.NewClient(wechatweb.Config{
		AppID:                providerConfig.AppID,
		AppSecret:            providerConfig.AppSecret,
		ConnectBaseURL:       "",
		WebRedirectURI:       providerConfig.RedirectURI,
		LoginScope:           providerConfig.Scope,
		RequestTimeoutSecond: 0,
	})
}

func newWeChatMiniProgramProviderClient(providerConfig *model.AuthProviderConfig) *wechatminiprogram.Client {
	return wechatminiprogram.NewClient(wechatminiprogram.Config{
		AppID:                providerConfig.AppID,
		AppSecret:            providerConfig.AppSecret,
		RequestTimeoutSecond: 0,
	})
}
