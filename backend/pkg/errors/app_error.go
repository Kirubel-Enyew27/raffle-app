package errors

import (
	"net/http"
)

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

func New(code, message string, httpStatus int) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		HTTPStatus: httpStatus,
	}
}

func WithField(code, message string, httpStatus int, err error) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		HTTPStatus: httpStatus,
		Err:       err,
	}
}

func IsCode(err error, code string) bool {
	if err == nil {
		return false
	}
	for {
		if a, ok := err.(*AppError); ok {
			if a.Code == code {
				return true
			}
		}
		appErr, ok := err.(*AppError)
		if !ok {
			break
		}
		err = appErr.Err
	}
	return false
}

func HTTPStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	for {
		if a, ok := err.(*AppError); ok {
			return a.HTTPStatus
		}
		appErr, ok := err.(*AppError)
		if !ok {
			break
		}
		err = appErr.Err
	}
	return http.StatusInternalServerError
}

func Wrap(err error, code, message string, httpStatus int) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		HTTPStatus: httpStatus,
		Err:       err,
	}
}

var (
	ErrNotFound         = &AppError{Code: "NOT_FOUND", Message: "Resource not found", HTTPStatus: http.StatusNotFound}
	ErrUnauthorized     = &AppError{Code: "UNAUTHORIZED", Message: "Unauthorized access", HTTPStatus: http.StatusUnauthorized}
	ErrForbidden        = &AppError{Code: "FORBIDDEN", Message: "Access forbidden", HTTPStatus: http.StatusForbidden}
	ErrValidationFailed = &AppError{Code: "VALIDATION_FAILED", Message: "Validation failed", HTTPStatus: http.StatusBadRequest}
	ErrConflict         = &AppError{Code: "CONFLICT", Message: "Resource conflict", HTTPStatus: http.StatusConflict}
	ErrRateLimited      = &AppError{Code: "RATE_LIMITED", Message: "Rate limit exceeded", HTTPStatus: http.StatusTooManyRequests}
	ErrInternal         = &AppError{Code: "INTERNAL_ERROR", Message: "Internal server error", HTTPStatus: http.StatusInternalServerError}
	ErrInsufficientFunds = &AppError{Code: "INSUFFICIENT_FUNDS", Message: "Insufficient funds", HTTPStatus: http.StatusBadRequest}
)
