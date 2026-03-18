package middleware

import (
	"time"

	"github.com/darren-you/auth_service/template_server/internal/constant"
	"github.com/darren-you/auth_service/template_server/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

type LoggerMiddleware struct{}

func NewLoggerMiddleware() *LoggerMiddleware {
	return &LoggerMiddleware{}
}

func (m *LoggerMiddleware) Logger(c *fiber.Ctx) error {
	start := time.Now()
	traceID := c.Get(constant.RequestIDHeader)
	if traceID == "" {
		if id, ok := c.Locals(constant.TraceIDKey).(string); ok {
			traceID = id
		}
	}

	body := c.Body()
	params := c.OriginalURL()
	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodDelete {
		if len(body) > 0 {
			params = string(body)
			if len(params) > 1024 {
				params = params[:1024] + "...[truncated]"
			}
		}
	}

	err := c.Next()
	logger.InfofWithTraceID(traceID,
		"Request received - Method: %s, Path: %s, Params: %s, Status: %d, Duration: %v, IP: %s, User-Agent: %s",
		c.Method(),
		c.Path(),
		params,
		c.Response().StatusCode(),
		time.Since(start),
		c.IP(),
		c.Get("User-Agent"),
	)

	return err
}
