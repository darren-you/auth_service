package router

import (
	"time"

	"github.com/darren-you/auth_service/template_server/internal/api/handler"
	"github.com/darren-you/auth_service/template_server/internal/api/middleware"
	"github.com/darren-you/auth_service/template_server/internal/service"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App, authService service.AuthService) {
	authHandler := handler.NewAuthHandler(authService)
	authMiddleware := middleware.NewAuthMiddleware(authService, authService.SessionConfig())

	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"code":      200,
			"timestamp": time.Now().UnixMilli(),
			"msg":       "ok",
			"data": fiber.Map{
				"service": "auth_service",
			},
		})
	})

	auth := api.Group("/auth")
	auth.Get("/providers/:provider/login-url", authHandler.GetLoginURL)
	auth.Post("/providers/:provider/callback", authHandler.ProviderCallback)
	auth.Post("/providers/phone/send-captcha", authHandler.SendPhoneCaptcha)
	auth.Post("/providers/guest/device-id", authHandler.IssueGuestDeviceID)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Post("/logout", authHandler.Logout)
	auth.Get("/me", authMiddleware.RequireAuth, authHandler.Me)
}
