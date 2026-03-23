package responsex

import (
	"context"
	"time"
)

const okCode = 200

type Envelope struct {
	Code      int    `json:"code"`
	Timestamp int64  `json:"timestamp"`
	Msg       string `json:"msg"`
	Data      any    `json:"data,omitempty"`
}

func New(code int, msg string, data any) Envelope {
	return Envelope{
		Code:      code,
		Timestamp: time.Now().UnixMilli(),
		Msg:       msg,
		Data:      data,
	}
}

func OkHandler(_ context.Context, value any) any {
	return New(okCode, "ok", value)
}
