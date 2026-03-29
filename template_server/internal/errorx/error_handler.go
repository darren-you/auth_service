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
		message := strings.TrimSpace(customErr.Error())
		if message == "" {
			message = customErr.Message
		}

		if customErr.Err != nil {
			logx.WithContext(ctx).Errorw(
				"request failed",
				logx.Field("error.code", customErr.Code),
				logx.Field("http.status_code", customErr.HTTPStatus),
				logx.Field("error.message", message),
				logx.Field("error.detail", customErr.Err.Error()),
			)
		} else {
			logx.WithContext(ctx).Errorw(
				"request failed",
				logx.Field("error.code", customErr.Code),
				logx.Field("http.status_code", customErr.HTTPStatus),
				logx.Field("error.message", message),
			)
		}
		return customErr.HTTPStatus, responsex.New(customErr.Code, message, nil)
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = ErrBadRequest.Message
	}

	logx.WithContext(ctx).Errorw(
		"request failed",
		logx.Field("error.code", ErrBadRequest.Code),
		logx.Field("http.status_code", http.StatusBadRequest),
		logx.Field("error.message", message),
		logx.Field("error.detail", err.Error()),
	)
	return http.StatusBadRequest, responsex.New(ErrBadRequest.Code, message, nil)
}
