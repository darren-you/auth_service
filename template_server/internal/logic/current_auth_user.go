package logic

import (
	"context"
	"strings"

	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/middleware"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
)

type currentAuthUserContext struct {
	accessIdentity *middleware.AccessIdentity
	authUser       *model.AuthUser
	tenant         *model.AuthTenant
}

func resolveCurrentAuthUserContext(ctx context.Context, svcCtx *svc.ServiceContext) (*currentAuthUserContext, error) {
	accessIdentity, err := middleware.CurrentAccessIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}

	tenant, err := svcCtx.AuthRepo.FindTenantByKey(ctx, accessIdentity.TenantKey)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil {
		return nil, appErrors.ErrTenantNotFound
	}

	authUserID := accessIdentity.AuthUserID
	if authUserID == 0 {
		userByTokenUserID, findErr := svcCtx.AuthRepo.FindUserByTokenUserID(ctx, tenant.ID, accessIdentity.TokenUserID)
		if findErr != nil {
			return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, findErr)
		}
		if userByTokenUserID != nil {
			authUserID = userByTokenUserID.ID
		}
	}
	if authUserID == 0 {
		sessionRecord, sessionErr := svcCtx.AuthRepo.FindLatestActiveSessionByTokenUserID(ctx, tenant.ID, accessIdentity.TokenUserID)
		if sessionErr != nil {
			return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, sessionErr)
		}
		if sessionRecord == nil || sessionRecord.AuthUserID == 0 {
			return nil, appErrors.ErrUnauthorized
		}
		authUserID = sessionRecord.AuthUserID
	}

	user, err := svcCtx.AuthRepo.FindUserByID(ctx, authUserID)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if user == nil || user.TenantID != tenant.ID {
		return nil, appErrors.ErrUnauthorized
	}

	return &currentAuthUserContext{
		accessIdentity: accessIdentity,
		authUser:       user,
		tenant:         tenant,
	}, nil
}

func buildCurrentAuthUserResp(current *currentAuthUserContext) *types.AuthUserResp {
	if current == nil || current.accessIdentity == nil || current.authUser == nil || current.tenant == nil {
		return nil
	}

	var lastLoginAt int64
	if current.authUser.LastLoginAt != nil {
		lastLoginAt = current.authUser.LastLoginAt.UnixMilli()
	}

	return &types.AuthUserResp{
		Id:          uint64(current.accessIdentity.TokenUserID),
		TenantKey:   current.tenant.TenantKey,
		DisplayName: strings.TrimSpace(current.authUser.DisplayName),
		AvatarURL:   strings.TrimSpace(current.authUser.AvatarURL),
		Role:        strings.TrimSpace(current.authUser.Role),
		Status:      strings.TrimSpace(current.authUser.Status),
		LastLoginAt: lastLoginAt,
	}
}
