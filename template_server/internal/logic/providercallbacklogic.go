// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProviderCallbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProviderCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProviderCallbackLogic {
	return &ProviderCallbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProviderCallbackLogic) ProviderCallback(req *types.ProviderCallbackReq) (resp *types.SessionResp, err error) {
	return newAuthFlow(l.ctx, l.svcCtx).ProviderCallback(req)
}
