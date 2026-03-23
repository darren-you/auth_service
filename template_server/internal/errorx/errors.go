package errorx

import (
	stdErrors "errors"
	"net/http"
)

type CustomError struct {
	Code       int
	HTTPStatus int
	Message    string
	Err        error
}

func (e *CustomError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *CustomError) Unwrap() error {
	return e.Err
}

func New(code, httpStatus int, message string, err error) *CustomError {
	return &CustomError{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
		Err:        err,
	}
}

func NewWithStatus(code, httpStatus int, message string) *CustomError {
	return New(code, httpStatus, message, nil)
}

func Is(err error, target *CustomError) bool {
	var customErr *CustomError
	if stdErrors.As(err, &customErr) {
		return customErr.Code == target.Code
	}
	return false
}

func IsCustomError(err error) (bool, *CustomError) {
	var customErr *CustomError
	ok := stdErrors.As(err, &customErr)
	return ok, customErr
}

var (
	ErrBadRequest          = NewWithStatus(4000, http.StatusBadRequest, "bad request")
	ErrUnauthorized        = NewWithStatus(4001, http.StatusUnauthorized, "unauthorized")
	ErrForbidden           = NewWithStatus(4003, http.StatusForbidden, "forbidden")
	ErrNotFound            = NewWithStatus(4004, http.StatusNotFound, "not found")
	ErrInternalServer      = NewWithStatus(5000, http.StatusInternalServerError, "internal server error")
	ErrTenantNotFound      = NewWithStatus(4101, http.StatusNotFound, "tenant not found")
	ErrProviderNotEnabled  = NewWithStatus(4102, http.StatusBadRequest, "provider not enabled")
	ErrUnsupportedProvider = NewWithStatus(4103, http.StatusBadRequest, "unsupported provider")
	ErrWeChatStateInvalid  = NewWithStatus(4104, http.StatusBadRequest, "wechat login state invalid")
	ErrAuthFailed          = NewWithStatus(4105, http.StatusBadRequest, "authentication failed")
	ErrTokenInvalid        = NewWithStatus(4106, http.StatusUnauthorized, "invalid token")
	ErrTokenExpired        = NewWithStatus(4107, http.StatusUnauthorized, "token expired")
	ErrCaptchaInvalid      = NewWithStatus(4108, http.StatusBadRequest, "invalid captcha")
	ErrSessionRevoked      = NewWithStatus(4109, http.StatusUnauthorized, "session revoked")
	ErrConfigInvalid       = NewWithStatus(4110, http.StatusBadRequest, "provider configuration invalid")
)
