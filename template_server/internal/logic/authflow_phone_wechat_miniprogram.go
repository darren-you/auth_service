package logic

import (
	"fmt"
	"strings"

	"github.com/darren-you/auth_service/providerkeys"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/model"
)

func (s *authFlow) loginWithPhoneMiniProgramCode(
	tenant *model.AuthTenant,
	providerConfig *model.AuthProviderConfig,
	req *ProviderCallbackRequest,
) (*SessionResponse, error) {
	phoneCode := strings.TrimSpace(req.PhoneCode)
	if phoneCode == "" {
		return nil, appErrors.ErrBadRequest
	}

	_, wechatProviderConfig, err := s.resolveTenantAndProvider(
		req.TenantKey,
		providerkeys.ProviderWeChatMiniProgram,
		providerkeys.ClientTypeMiniProgram,
	)
	if err != nil {
		return nil, err
	}

	client := newWeChatMiniProgramProviderClient(wechatProviderConfig)
	phoneInfo, err := client.GetPhoneNumberByCode(s.ctx, phoneCode)
	if err != nil {
		s.Errorf(
			"wechat miniprogram phone code exchange failed: tenant=%s code=%s err=%v",
			tenant.TenantKey,
			maskTail(phoneCode, 6),
			err,
		)
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			appErrors.ErrAuthFailed.Message,
			fmt.Errorf("exchange wechat miniprogram phone code failed: %w", err),
		)
	}

	phone := strings.TrimSpace(phoneInfo.PurePhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(phoneInfo.PhoneNumber)
	}
	if phone == "" {
		return nil, appErrors.New(
			appErrors.ErrAuthFailed.Code,
			appErrors.ErrAuthFailed.HTTPStatus,
			"wechat phone number is empty",
			nil,
		)
	}

	return s.issuePhoneSession(tenant, providerConfig, phone, req.DisplayName, req.AvatarURL, uint(req.CurrentUserID), req.CurrentUserRole, map[string]string{
		"phone":        phone,
		"login_method": "wechat_miniprogram_phone",
	})
}
