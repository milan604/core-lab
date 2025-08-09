# apperr — Application Error Model

Structured, composable application errors with canonical codes, HTTP status mapping, and fluent helpers. Designed to pair with `pkg/response` and Gin middleware for consistent API error shapes.

## Concepts
- ErrorCode: canonical code + human message + HTTP status
- AppError: serializable error with code/message/status and optional field suggestions
- Fluent helpers: build and enrich errors ergonomically

## Quick Start
```go
import (
  "corelab/pkg/apperr"
)

// Build from a code
err := apperr.New(apperr.ErrorCodeInvalidInput).
  AddSuggestion("email", "must be a valid email")

// Override message or status
err = err.WithMessage("validation error").WithStatus(422)

// Wrap a cause
cause := fmt.Errorf("db timeout")
err = apperr.New(apperr.ErrorCodeInternal).Wrap(cause)
```

## JSON Shape
```json
{
  "code": "validation_failed",
  "message": "Validation failed",
  "suggestions": [
    {"field": "email", "message": "must be a valid email"}
  ]
}
```

## API
- `New(code *ErrorCode) *AppError`
- `Newf(code *ErrorCode, format string, args ...any) *AppError`
- `FromError(err error) *AppError` — wraps unknown errors as internal
- `(*AppError) AddSuggestion(field, msg string) *AppError`
- `(*AppError) WithStatus(status int) *AppError`
- `(*AppError) WithMessage(msg string) *AppError`
- `(*AppError) WithCode(code *ErrorCode) *AppError`
- `(*AppError) Wrap(err error) *AppError`
- `(*AppError) Unwrap() error`

## ErrorCode
```go
var (
  ErrorCodeSuccess        = apperr.NewErrorCode("success", "OK", 0, 200)
  ErrorCodeInvalidRequest = apperr.NewErrorCode("invalid_request", "Invalid request body", 10, 400)
  // ...extend as needed
)
```

## With Gin
Use the error handler middleware in `pkg/server/middleware/errorhandler.go` and return `*apperr.AppError` from handlers or let `pkg/validator` produce them.

```go
// handler example
if bad {
  return c.Error(apperr.New(apperr.ErrorCodeForbidden))
}
```

## Tips
- Keep messages user-friendly; log technical details via your logger.
- Prefer `ErrorCode` for consistent API behavior.
- Attach field-level suggestions for validation problems.

---
Private and proprietary. All rights reserved.
