package observability

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	RequestIDHeader = "X-Request-ID"
	TraceIDKey      = "trace_id"
)

type traceIDContextKey struct{}

func NormalizeOrNewRequestID(raw string) string {
	value := strings.TrimSpace(raw)
	if value != "" {
		return value
	}

	return uuid.NewString()
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	normalized := strings.TrimSpace(traceID)
	if normalized == "" {
		return ctx
	}

	ctx = context.WithValue(ctx, traceIDContextKey{}, normalized)
	return logx.ContextWithFields(ctx, logx.Field(TraceIDKey, normalized))
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if value, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return strings.TrimSpace(value)
	}

	return ""
}

func PropagateRequestID(req *http.Request, ctx context.Context) {
	if req == nil || ctx == nil {
		return
	}

	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		return
	}

	req.Header.Set(RequestIDHeader, traceID)
}
