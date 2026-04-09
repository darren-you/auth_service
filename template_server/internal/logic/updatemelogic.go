package logic

import (
	"context"
	"strings"

	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMeLogic {
	return &UpdateMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMeLogic) UpdateMe(req *types.UpdateMeReq) (resp *types.AuthUserResp, err error) {
	current, err := resolveCurrentAuthUserContext(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if displayName == "" && avatarURL == "" {
		return nil, appErrors.ErrBadRequest
	}

	updatedDisplayName := current.authUser.DisplayName
	if displayName != "" {
		updatedDisplayName = displayName
	}
	updatedAvatarURL := current.authUser.AvatarURL
	if avatarURL != "" {
		updatedAvatarURL = avatarURL
	}

	if updatedDisplayName == current.authUser.DisplayName && updatedAvatarURL == current.authUser.AvatarURL {
		return buildCurrentAuthUserResp(current), nil
	}

	if err := l.svcCtx.AuthRepo.UpdateUserProfileAndActiveSessions(
		l.ctx,
		current.authUser.ID,
		updatedDisplayName,
		updatedAvatarURL,
		current.authUser.Role,
		current.authUser.Status,
	); err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	current.authUser.DisplayName = updatedDisplayName
	current.authUser.AvatarURL = updatedAvatarURL

	return buildCurrentAuthUserResp(current), nil
}
