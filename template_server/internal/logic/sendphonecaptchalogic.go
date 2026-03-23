// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendPhoneCaptchaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendPhoneCaptchaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendPhoneCaptchaLogic {
	return &SendPhoneCaptchaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendPhoneCaptchaLogic) SendPhoneCaptcha(req *types.PhoneCaptchaSendReq) (resp *types.PhoneCaptchaSendResp, err error) {
	return newAuthFlow(l.ctx, l.svcCtx).SendPhoneCaptcha(req)
}
