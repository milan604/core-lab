// Package apperr defines a small, composable application error model with
// canonical codes, HTTP status mapping, and fluent helpers for building
// structured errors to return from handlers and middleware.
package apperr

import (
	"fmt"
)

// Suggestion is a per-field suggestion to fix a validation error.
type Suggestion struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// AppError is the canonical error shape used across handlers and serialized to clients.
type AppError struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	Suggestions []Suggestion `json:"suggestions,omitempty"` // useful for validation errors
	HTTPStatus  int          `json:"-"`
	cause       error        `json:"-"`
	ec          *ErrorCode   `json:"-"`
}

// New creates a new AppError from an ErrorCode.
func New(ec *ErrorCode) *AppError {
	if ec == nil {
		ec = ErrorCodeInternal
	}
	return &AppError{
		Code:       ec.Code(),
		Message:    ec.Message(),
		HTTPStatus: ec.HTTPStatus(),
		ec:         ec,
	}
}

// Newf creates AppError with formatted message.
func Newf(ec *ErrorCode, format string, args ...interface{}) *AppError {
	a := New(ec)
	a.Message = fmt.Sprintf(format, args...)
	return a
}

// FromError wraps a generic error into AppError (internal fallback)
func FromError(err error) *AppError {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*AppError); ok {
		return ae
	}
	// unknown error -> internal server error
	a := New(ErrorCodeInternal)
	a.cause = err
	// keep message minimal for clients
	a.Message = ErrorCodeInternal.Message()
	return a
}

// AddSuggestion appends a field suggestion (fluent)
func (a *AppError) AddSuggestion(field, message string) *AppError {
	if a == nil {
		a = New(ErrorCodeInternal)
	}
	a.Suggestions = append(a.Suggestions, Suggestion{
		Field:   field,
		Message: message,
	})
	return a
}

func (a *AppError) Error() string {
	if a == nil {
		return "<nil>"
	}
	if a.cause != nil {
		return a.cause.Error()
	}
	return a.Message
}

// WithStatus sets/overrides the HTTP status and returns the same AppError for chaining.
func (a *AppError) WithStatus(status int) *AppError {
	if a == nil {
		return New(ErrorCodeInternal).WithStatus(status)
	}
	a.HTTPStatus = status
	return a
}

// WithMessage overrides the message and returns the same AppError for chaining.
func (a *AppError) WithMessage(msg string) *AppError {
	if a == nil {
		return New(ErrorCodeInternal).WithMessage(msg)
	}
	a.Message = msg
	return a
}

// WithCode replaces the underlying ErrorCode (code/message/status) and returns the AppError.
func (a *AppError) WithCode(ec *ErrorCode) *AppError {
	if ec == nil {
		ec = ErrorCodeInternal
	}
	if a == nil {
		return New(ec)
	}
	a.ec = ec
	a.Code = ec.Code()
	a.Message = ec.Message()
	a.HTTPStatus = ec.HTTPStatus()
	return a
}

// Wrap sets the underlying cause and returns the same AppError.
func (a *AppError) Wrap(err error) *AppError {
	if a == nil {
		a = New(ErrorCodeInternal)
	}
	a.cause = err
	return a
}

// Unwrap returns the underlying cause, allowing errors.Unwrap/Is/As to work.
func (a *AppError) Unwrap() error { return a.cause }

// HasError returns true if the error is not nil and is not an empty AppError
func HasError(err error) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*AppError); ok {
		return ae.Code != "" || ae.Message != ""
	}
	return true
}

// HasErrors returns true if the AppError has a code, message, or suggestions
func (a *AppError) HasErrors() bool {
	if a == nil {
		return false
	}
	return a.Code != "" || a.Message != "" || len(a.Suggestions) > 0
}
