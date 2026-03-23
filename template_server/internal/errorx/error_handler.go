package errorx

import (
	"context"
	"net/http"
	"strings"

	"github.com/darren-you/auth_service/template_server/pkg/responsex"
)

func Handler(_ context.Context, err error) (int, any) {
	if ok, customErr := IsCustomError(err); ok {
		return customErr.HTTPStatus, responsex.New(customErr.Code, customErr.Message, nil)
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = ErrBadRequest.Message
	}

	return http.StatusBadRequest, responsex.New(ErrBadRequest.Code, message, nil)
}
