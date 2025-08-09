package apperr

import "net/http"

// Predefined standard error codes (can be extended)
var (
	ErrorCodeSuccess        = NewErrorCode("success", "OK", 0, http.StatusOK)
	ErrorCodeInvalidRequest = NewErrorCode("invalid_request", "Invalid request body", 10, http.StatusBadRequest)
	ErrorCodeInvalidInput   = NewErrorCode("invalid_input", "Invalid input", 20, http.StatusUnprocessableEntity)
	ErrorCodeValidationFail = NewErrorCode("validation_failed", "Validation failed", 30, http.StatusUnprocessableEntity)
	ErrorCodeUnauthorized   = NewErrorCode("unauthorized", "Unauthorized", 40, http.StatusUnauthorized)
	ErrorCodeForbidden      = NewErrorCode("forbidden", "Forbidden", 50, http.StatusForbidden)
	ErrorCodeNotFound       = NewErrorCode("not_found", "Not found", 60, http.StatusNotFound)
	ErrorCodeInternal       = NewErrorCode("internal_error", "Internal server error", 100, http.StatusInternalServerError)
)

// ErrorCode describes a canonical application error code.
// It carries a numeric severity/priority (Value) and an HTTP status.
type ErrorCode struct {
	code       string
	message    string
	value      int
	httpStatus int
}

func NewErrorCode(code, message string, value, httpStatus int) *ErrorCode {
	return &ErrorCode{code: code, message: message, value: value, httpStatus: httpStatus}
}

func (ec *ErrorCode) Code() string    { return ec.code }
func (ec *ErrorCode) Message() string { return ec.message }
func (ec *ErrorCode) Value() int      { return ec.value }
func (ec *ErrorCode) HTTPStatus() int { return ec.httpStatus }
