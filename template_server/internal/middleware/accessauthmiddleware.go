package middleware

import (
	"context"
	"net/http"

	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
	"github.com/darren-you/auth_service/template_server/pkg/session"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type currentUserContextKey struct{}

type AccessAuthMiddleware struct {
	sessionConfig session.Config
}

func NewAccessAuthMiddleware(svcCtx *svc.ServiceContext) *AccessAuthMiddleware {
	return &AccessAuthMiddleware{
		sessionConfig: svcCtx.SessionConfig,
	}
}

func (m *AccessAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := session.ExtractBearerToken(r.Header.Get("Authorization"))
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, appErrors.ErrUnauthorized)
			return
		}

		claims, err := session.ParseAccessToken(token, m.sessionConfig)
		if err != nil {
			switch err {
			case session.ErrExpiredToken:
				httpx.ErrorCtx(r.Context(), w, appErrors.ErrTokenExpired)
			default:
				httpx.ErrorCtx(r.Context(), w, appErrors.ErrTokenInvalid)
			}
			return
		}

		currentUser := &types.AuthUserResp{
			Id:          uint64(claims.UserID),
			TenantKey:   claims.TenantKey,
			DisplayName: claims.Username,
			AvatarURL:   claims.AvatarURL,
			Role:        claims.Role,
			Status:      profileStatus(claims.Status),
		}
		ctx := context.WithValue(r.Context(), currentUserContextKey{}, currentUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func CurrentUserFromContext(ctx context.Context) (*types.AuthUserResp, error) {
	currentUser, ok := ctx.Value(currentUserContextKey{}).(*types.AuthUserResp)
	if !ok || currentUser == nil {
		return nil, appErrors.ErrUnauthorized
	}
	return currentUser, nil
}

func profileStatus(status string) string {
	if status == "" {
		return "active"
	}
	return status
}
