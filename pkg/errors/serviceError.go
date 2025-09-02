package errors

import (
	stdErrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/milan604/core-lab/pkg/apperr"
)

// ServiceError is an advanced, structured service-layer error.
// It carries a stable code, human message, HTTP status, and rich context.
type ServiceError struct {
	Code          string              `json:"code"`
	Message       string              `json:"message"`
	HTTPStatus    int                 `json:"-"`
	ID            string              `json:"id,omitempty"`
	CorrelationID string              `json:"correlation_id,omitempty"`
	Details       map[string]any      `json:"details,omitempty"`
	Suggestions   []apperr.Suggestion `json:"suggestions,omitempty"`
	Retryable     bool                `json:"-"`
	Timestamp     time.Time           `json:"timestamp"`
	cause         error               `json:"-"`
}

// Option configures a ServiceError.
type Option func(*ServiceError)

// WithID sets a stable error ID (for logs/tracing).
func WithID(id string) Option { return func(se *ServiceError) { se.ID = id } }

// WithCorrelation sets correlation/request id.
func WithCorrelation(id string) Option { return func(se *ServiceError) { se.CorrelationID = id } }

// WithDetail adds a single detail key-value.
func WithDetail(key string, val any) Option {
	return func(se *ServiceError) {
		if se.Details == nil {
			se.Details = make(map[string]any)
		}
		se.Details[key] = val
	}
}

// WithDetails merges given details map.
func WithDetails(m map[string]any) Option {
	return func(se *ServiceError) {
		if len(m) == 0 {
			return
		}
		if se.Details == nil {
			se.Details = make(map[string]any)
		}
		for k, v := range m {
			se.Details[k] = v
		}
	}
}

// WithSuggestion appends a suggestion (typically for validation).
func WithSuggestion(field, message string) Option {
	return func(se *ServiceError) {
		se.Suggestions = append(se.Suggestions, apperr.Suggestion{Field: field, Message: message})
	}
}

// WithRetryable marks error as retryable.
func WithRetryable(retryable bool) Option { return func(se *ServiceError) { se.Retryable = retryable } }

// WithCause sets underlying cause.
func WithCause(err error) Option { return func(se *ServiceError) { se.cause = err } }

// WithStatus overrides HTTP status.
func WithStatus(status int) Option { return func(se *ServiceError) { se.HTTPStatus = status } }

// WithMessage overrides message.
func WithMessage(msg string) Option { return func(se *ServiceError) { se.Message = msg } }

// NewServiceError constructs a ServiceError with functional options.
func NewServiceError(code, message string, httpStatus int, opts ...Option) *ServiceError {
	se := &ServiceError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Timestamp:  time.Now().UTC(),
	}
	for _, o := range opts {
		o(se)
	}
	return se
}

// FromCode creates ServiceError from a canonical ErrorCode.
func FromCode(ec *apperr.ErrorCode, opts ...Option) *ServiceError {
	if ec == nil {
		ec = apperr.ErrorCodeInternal
	}
	return NewServiceError(ec.Code(), ec.Message(), ec.HTTPStatus(), opts...)
}

// Common helpers.
func Internal(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeInternal, append([]Option{WithMessage(msg)}, opts...)...)
}
func NotFound(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeNotFound, append([]Option{WithMessage(msg)}, opts...)...)
}
func Forbidden(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeForbidden, append([]Option{WithMessage(msg)}, opts...)...)
}
func Unauthorized(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeUnauthorized, append([]Option{WithMessage(msg)}, opts...)...)
}
func BadRequest(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeInvalidRequest, append([]Option{WithMessage(msg)}, opts...)...)
}

func InvalidInput(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeInvalidInput, append([]Option{WithMessage(msg)}, opts...)...)
}

func ValidationFailed(msg string, opts ...Option) *ServiceError {
	return FromCode(apperr.ErrorCodeValidationFail, append([]Option{WithMessage(msg)}, opts...)...)
}

func Conflict(msg string, opts ...Option) *ServiceError {
	return NewServiceError("conflict", msg, http.StatusConflict, opts...)
}

func AlreadyExists(msg string, opts ...Option) *ServiceError {
	return NewServiceError("already_exists", msg, http.StatusConflict, opts...)
}

func TooManyRequests(msg string, opts ...Option) *ServiceError {
	return NewServiceError("too_many_requests", msg, http.StatusTooManyRequests, opts...)
}

// Alias for TooManyRequests
func RateLimited(msg string, opts ...Option) *ServiceError { return TooManyRequests(msg, opts...) }

func ServiceUnavailable(msg string, opts ...Option) *ServiceError {
	return NewServiceError("service_unavailable", msg, http.StatusServiceUnavailable, opts...)
}

func BadGateway(msg string, opts ...Option) *ServiceError {
	return NewServiceError("bad_gateway", msg, http.StatusBadGateway, opts...)
}

func GatewayTimeout(msg string, opts ...Option) *ServiceError {
	return NewServiceError("gateway_timeout", msg, http.StatusGatewayTimeout, opts...)
}

// Alias for GatewayTimeout
func Timeout(msg string, opts ...Option) *ServiceError { return GatewayTimeout(msg, opts...) }

func RequestTimeout(msg string, opts ...Option) *ServiceError {
	return NewServiceError("request_timeout", msg, http.StatusRequestTimeout, opts...)
}

func MethodNotAllowed(msg string, opts ...Option) *ServiceError {
	return NewServiceError("method_not_allowed", msg, http.StatusMethodNotAllowed, opts...)
}

func NotAcceptable(msg string, opts ...Option) *ServiceError {
	return NewServiceError("not_acceptable", msg, http.StatusNotAcceptable, opts...)
}

func UnsupportedMediaType(msg string, opts ...Option) *ServiceError {
	return NewServiceError("unsupported_media_type", msg, http.StatusUnsupportedMediaType, opts...)
}

func UnprocessableEntity(msg string, opts ...Option) *ServiceError {
	return NewServiceError("unprocessable_entity", msg, http.StatusUnprocessableEntity, opts...)
}

func PreconditionFailed(msg string, opts ...Option) *ServiceError {
	return NewServiceError("precondition_failed", msg, http.StatusPreconditionFailed, opts...)
}

func NotImplemented(msg string, opts ...Option) *ServiceError {
	return NewServiceError("not_implemented", msg, http.StatusNotImplemented, opts...)
}

// ParseServiceError extracts a ServiceError from err or creates an Internal one.
func ParseServiceError(err error) *ServiceError {
	if err == nil {
		return nil
	}
	// If already a *ServiceError
	var se *ServiceError
	if stdErrors.As(err, &se) {
		return se
	}
	// If it's an AppError, map it
	if ae, ok := err.(*apperr.AppError); ok {
		out := NewServiceError(ae.Code, ae.Message, ae.HTTPStatus)
		out.Suggestions = ae.Suggestions
		return out
	}
	// Fallback: wrap as internal
	return Internal(err.Error(), WithCause(err))
}

// Error implements the error interface.
func (se *ServiceError) Error() string {
	if se == nil {
		return ""
	}
	if se.cause != nil {
		return fmt.Sprintf("%s: %s: %v", se.Code, se.Message, se.cause)
	}
	return fmt.Sprintf("%s: %s", se.Code, se.Message)
}

// Unwrap enables errors.Is/As on underlying cause.
func (se *ServiceError) Unwrap() error { return se.cause }

// IsCode reports whether this error has the given code.
func (se *ServiceError) IsCode(code string) bool { return se != nil && se.Code == code }

// Fluent mutators.
func (se *ServiceError) WithDetail(key string, val any) *ServiceError {
	WithDetail(key, val)(se)
	return se
}
func (se *ServiceError) WithDetails(m map[string]any) *ServiceError { WithDetails(m)(se); return se }
func (se *ServiceError) WithSuggestion(field, msg string) *ServiceError {
	WithSuggestion(field, msg)(se)
	return se
}
func (se *ServiceError) WithCause(err error) *ServiceError    { WithCause(err)(se); return se }
func (se *ServiceError) WithStatus(status int) *ServiceError  { WithStatus(status)(se); return se }
func (se *ServiceError) WithMessage(msg string) *ServiceError { WithMessage(msg)(se); return se }

// Accessors (kept for compatibility)
func (se *ServiceError) GetCode() string {
	if se == nil {
		return ""
	}
	return se.Code
}
func (se *ServiceError) GetMessage() string {
	if se == nil {
		return ""
	}
	return se.Message
}
func (se *ServiceError) GetHTTPStatus() int {
	if se == nil {
		return 0
	}
	return se.HTTPStatus
}

// ToAppError converts ServiceError into apperr.AppError for response serialization.
func (se *ServiceError) ToAppError() *apperr.AppError {
	if se == nil {
		return nil
	}
	ec := apperr.NewErrorCode(se.Code, se.Message, 0, se.HTTPStatus)
	a := apperr.New(ec)
	a.Message = se.Message
	if len(se.Suggestions) > 0 {
		a.Suggestions = append([]apperr.Suggestion(nil), se.Suggestions...)
	}
	return a
}
