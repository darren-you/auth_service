package middleware

import (
	"context"

	"github.com/darren-you/auth_service/template_server/internal/constant"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TraceMiddleware struct{}

func NewTraceMiddleware() *TraceMiddleware {
	return &TraceMiddleware{}
}

func (m *TraceMiddleware) Trace(c *fiber.Ctx) error {
	traceID := c.Get(constant.RequestIDHeader)
	if traceID == "" {
		traceID = uuid.NewString()
	}

	c.Locals(constant.TraceIDKey, traceID)
	c.Set(constant.RequestIDHeader, traceID)
	c.SetUserContext(context.WithValue(c.UserContext(), constant.TraceIDKey, traceID))
	return c.Next()
}
