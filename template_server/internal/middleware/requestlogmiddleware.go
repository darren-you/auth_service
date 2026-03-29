package middleware

import (
	"net/http"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/observability"
	"github.com/zeromicro/go-zero/core/logx"
)

type RequestLogMiddleware struct{}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func NewRequestLogMiddleware() *RequestLogMiddleware {
	return &RequestLogMiddleware{}
}

func (m *RequestLogMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := newStatusRecorder(w)

		next(recorder, r)

		traceID := observability.TraceIDFromContext(r.Context())
		logx.WithContext(r.Context()).Infow(
			"http request",
			logx.Field("trace_id", traceID),
			logx.Field("http.method", r.Method),
			logx.Field("http.route", r.URL.Path),
			logx.Field("http.status_code", recorder.statusCode),
			logx.Field("latency_ms", time.Since(start).Milliseconds()),
		)
	}
}
