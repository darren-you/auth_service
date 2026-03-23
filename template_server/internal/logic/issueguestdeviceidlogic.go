// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IssueGuestDeviceIDLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIssueGuestDeviceIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IssueGuestDeviceIDLogic {
	return &IssueGuestDeviceIDLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IssueGuestDeviceIDLogic) IssueGuestDeviceID(req *types.GuestDeviceIDReq) (resp *types.GuestDeviceIDResp, err error) {
	return newAuthFlow(l.ctx, l.svcCtx).IssueGuestDeviceID(req)
}
