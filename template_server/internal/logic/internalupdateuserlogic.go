package logic

import (
	"context"
	"strings"

	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type InternalUpdateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewInternalUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InternalUpdateUserLogic {
	return &InternalUpdateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InternalUpdateUserLogic) InternalUpdateUser(bridgeAuthKey string, req *types.InternalUpdateUserReq) (*types.AuthUserResp, error) {
	if req == nil || req.UserID == 0 {
		return nil, appErrors.ErrBadRequest
	}

	tenant, runtimeCfg, err := l.resolveTenantAndRuntime(req.TenantKey)
	if err != nil {
		return nil, err
	}
	if runtimeCfg.BridgeAuthKey == "" || strings.TrimSpace(bridgeAuthKey) != runtimeCfg.BridgeAuthKey {
		return nil, appErrors.ErrUnauthorized
	}

	user, err := l.svcCtx.AuthRepo.FindUserByTokenUserID(l.ctx, tenant.ID, uint(req.UserID))
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if user == nil || user.TenantID != tenant.ID {
		return nil, appErrors.ErrNotFound
	}

	updatedDisplayName := user.DisplayName
	if displayName := strings.TrimSpace(req.DisplayName); displayName != "" {
		updatedDisplayName = displayName
	}
	updatedAvatarURL := user.AvatarURL
	if avatarURL := strings.TrimSpace(req.AvatarURL); avatarURL != "" {
		updatedAvatarURL = avatarURL
	}
	updatedRole := user.Role
	if role := strings.TrimSpace(req.Role); role != "" {
		if !isSupportedUserRole(role) {
			return nil, appErrors.ErrBadRequest
		}
		updatedRole = role
	}
	updatedStatus := user.Status
	if status := strings.TrimSpace(req.Status); status != "" {
		if !isSupportedUserStatus(status) {
			return nil, appErrors.ErrBadRequest
		}
		updatedStatus = status
	}

	if updatedDisplayName == user.DisplayName &&
		updatedAvatarURL == user.AvatarURL &&
		updatedRole == user.Role &&
		updatedStatus == user.Status {
		return buildInternalAuthUserResp(tenant, user, uint(req.UserID)), nil
	}

	if err := l.svcCtx.AuthRepo.UpdateUserProfileAndActiveSessions(
		l.ctx,
		user.ID,
		updatedDisplayName,
		updatedAvatarURL,
		updatedRole,
		updatedStatus,
	); err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	user.DisplayName = updatedDisplayName
	user.AvatarURL = updatedAvatarURL
	user.Role = updatedRole
	user.Status = updatedStatus

	return buildInternalAuthUserResp(tenant, user, uint(req.UserID)), nil
}

func (l *InternalUpdateUserLogic) resolveTenantAndRuntime(tenantKey string) (*model.AuthTenant, *tenantRuntimeConfig, error) {
	normalizedTenantKey := normalize(tenantKey)
	if normalizedTenantKey == "" {
		return nil, nil, appErrors.ErrBadRequest
	}

	tenant, err := l.svcCtx.AuthRepo.FindTenantByKey(l.ctx, normalizedTenantKey)
	if err != nil {
		return nil, nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}
	if tenant == nil {
		return nil, nil, appErrors.ErrTenantNotFound
	}

	runtimeCfg, err := (&authFlow{ctx: l.ctx, svcCtx: l.svcCtx}).resolveTenantRuntimeConfig(normalizedTenantKey)
	if err != nil {
		return nil, nil, err
	}
	return tenant, runtimeCfg, nil
}

func buildInternalAuthUserResp(tenant *model.AuthTenant, user *model.AuthUser, tokenUserID uint) *types.AuthUserResp {
	if tenant == nil || user == nil {
		return nil
	}
	if tokenUserID == 0 {
		tokenUserID = user.TokenUserID
	}

	var lastLoginAt int64
	if user.LastLoginAt != nil {
		lastLoginAt = user.LastLoginAt.UnixMilli()
	}

	return &types.AuthUserResp{
		Id:          uint64(tokenUserID),
		TenantKey:   tenant.TenantKey,
		DisplayName: strings.TrimSpace(user.DisplayName),
		AvatarURL:   strings.TrimSpace(user.AvatarURL),
		Role:        strings.TrimSpace(user.Role),
		Status:      strings.TrimSpace(user.Status),
		LastLoginAt: lastLoginAt,
	}
}

func isSupportedUserRole(role string) bool {
	switch strings.TrimSpace(role) {
	case "admin", "user", "guest":
		return true
	default:
		return false
	}
}

func isSupportedUserStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "active", "disabled":
		return true
	default:
		return false
	}
}
