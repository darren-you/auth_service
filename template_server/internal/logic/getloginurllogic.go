// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLoginURLLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLoginURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLoginURLLogic {
	return &GetLoginURLLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLoginURLLogic) GetLoginURL(req *types.GetLoginURLReq) (resp *types.LoginURLResp, err error) {
	return newAuthFlow(l.ctx, l.svcCtx).GetLoginURL(req)
}
