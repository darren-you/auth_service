package middleware

import (
	"github.com/darren-you/auth_service/session"
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

	profile, err := m.authService.GetUserProfileByID(c.UserContext(), claims.UserID)
	if err != nil {
		return err
	}

	c.Locals("current_user", profile)
	return c.Next()
}
