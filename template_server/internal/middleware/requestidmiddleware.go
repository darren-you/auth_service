package middleware

import (
	"net/http"

	"github.com/darren-you/auth_service/template_server/internal/observability"
)

type RequestIDMiddleware struct{}

func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{}
}

func (m *RequestIDMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traceID := observability.NormalizeOrNewRequestID(r.Header.Get(observability.RequestIDHeader))
		w.Header().Set(observability.RequestIDHeader, traceID)
		r.Header.Set(observability.RequestIDHeader, traceID)

		ctx := observability.ContextWithTraceID(r.Context(), traceID)
		next(w, r.WithContext(ctx))
	}
}
