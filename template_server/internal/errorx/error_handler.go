package errorx

import (
	"context"
	"net/http"
	"strings"

	"github.com/darren-you/auth_service/template_server/pkg/responsex"
	"github.com/zeromicro/go-zero/core/logx"
)

func Handler(ctx context.Context, err error) (int, any) {
	if ok, customErr := IsCustomError(err); ok {
		if customErr.Err != nil {
			logx.WithContext(ctx).Errorf(
				"request failed: code=%d status=%d message=%s detail=%v",
				customErr.Code,
				customErr.HTTPStatus,
				customErr.Message,
				customErr.Err,
			)
		}

		message := strings.TrimSpace(customErr.Error())
		if message == "" {
			message = customErr.Message
		}
		return customErr.HTTPStatus, responsex.New(customErr.Code, message, nil)
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = ErrBadRequest.Message
	}

	logx.WithContext(ctx).Errorf("request failed: %v", err)
	return http.StatusBadRequest, responsex.New(ErrBadRequest.Code, message, nil)
}
