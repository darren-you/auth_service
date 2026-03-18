package middleware

import (
	"github.com/darren-you/auth_service/session"
	"github.com/darren-you/auth_service/template_server/internal/dto"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errors"
	"github.com/darren-you/auth_service/template_server/internal/service"
	"github.com/gofiber/fiber/v2"
)

type AuthMiddleware struct {
	authService   service.AuthService
	sessionConfig session.Config
}

func NewAuthMiddleware(authService service.AuthService, sessionConfig session.Config) *AuthMiddleware {
	return &AuthMiddleware{
		authService:   authService,
		sessionConfig: sessionConfig,
	}
}

func (m *AuthMiddleware) RequireAuth(c *fiber.Ctx) error {
	token, err := session.ExtractBearerToken(c.Get("Authorization"))
	if err != nil {
		return appErrors.ErrUnauthorized
	}

	claims, err := session.ParseAccessToken(token, m.sessionConfig)
	if err != nil {
		switch err {
		case session.ErrExpiredToken:
			return appErrors.ErrTokenExpired
		default:
			return appErrors.ErrTokenInvalid
		}
	}

	c.Locals("current_user", &dto.AuthUserResponse{
		ID:          claims.UserID,
		TenantKey:   claims.TenantKey,
		DisplayName: claims.Username,
		AvatarURL:   claims.AvatarURL,
		Role:        claims.Role,
		Status:      profileStatus(claims.Status),
	})
	return c.Next()
}

func profileStatus(status string) string {
	if status == "" {
		return "active"
	}
	return status
}
