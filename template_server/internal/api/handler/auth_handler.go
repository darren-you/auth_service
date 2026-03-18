package handler

import (
	"time"

	"github.com/darren-you/auth_service/template_server/internal/dto"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errors"
	"github.com/darren-you/auth_service/template_server/internal/service"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) GetLoginURL(c *fiber.Ctx) error {
	resp, err := h.authService.GetLoginURL(
		c.UserContext(),
		c.Query("tenant_key"),
		c.Params("provider"),
		c.Query("client_type"),
	)
	if err != nil {
		return err
	}
	return writeSuccess(c, resp)
}

func (h *AuthHandler) ProviderCallback(c *fiber.Ctx) error {
	var req dto.ProviderCallbackRequest
	if err := c.BodyParser(&req); err != nil {
		return appErrors.ErrBadRequest
	}
	resp, err := h.authService.ProviderCallback(c.UserContext(), c.Params("provider"), req)
	if err != nil {
		return err
	}
	return writeSuccess(c, resp)
}

func (h *AuthHandler) RegisterPassword(c *fiber.Ctx) error {
	var req dto.PasswordRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return appErrors.ErrBadRequest
	}
	resp, err := h.authService.RegisterPassword(c.UserContext(), req)
	if err != nil {
		return err
	}
	return writeSuccess(c, resp)
}

func (h *AuthHandler) SendPhoneCaptcha(c *fiber.Ctx) error {
	var req dto.PhoneCaptchaSendRequest
	if err := c.BodyParser(&req); err != nil {
		return appErrors.ErrBadRequest
	}
	resp, err := h.authService.SendPhoneCaptcha(c.UserContext(), req)
	if err != nil {
		return err
	}
	return writeSuccess(c, resp)
}

func (h *AuthHandler) IssueGuestDeviceID(c *fiber.Ctx) error {
	var req dto.GuestDeviceIDRequest
	if err := c.BodyParser(&req); err != nil {
		return appErrors.ErrBadRequest
	}
	resp, err := h.authService.IssueGuestDeviceID(c.UserContext(), req)
	if err != nil {
		return err
	}
	return writeSuccess(c, resp)
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req dto.RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return appErrors.ErrBadRequest
	}
	resp, err := h.authService.Refresh(c.UserContext(), req)
	if err != nil {
		return err
	}
	return writeSuccess(c, resp)
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req dto.LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return appErrors.ErrBadRequest
	}
	if err := h.authService.Logout(c.UserContext(), req); err != nil {
		return err
	}
	return writeSuccess(c, fiber.Map{"ok": true})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	profile, ok := c.Locals("current_user").(*dto.AuthUserResponse)
	if !ok || profile == nil {
		return appErrors.ErrUnauthorized
	}
	return writeSuccess(c, profile)
}

func writeSuccess(c *fiber.Ctx, data interface{}) error {
	return c.JSON(fiber.Map{
		"code":      200,
		"timestamp": time.Now().UnixMilli(),
		"msg":       "ok",
		"data":      data,
	})
}
