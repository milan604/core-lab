````markdown
# errors

Advanced error modeling for services:
- A lightweight error wrapper with stack traces (`Error`).
- A rich `ServiceError` type with codes, HTTP status, details, suggestions, correlation IDs, and conversions to your API error shape (`apperr.AppError`).

---

## Quick start

```go
import (
  serr "github.com/milan604/core-lab/pkg/errors"
)

// 1) Create a service error using canonical codes or helpers
err := serr.NotFound("user not found").WithDetail("user_id", id)

// 2) Convert to AppError for responses
appErr := err.ToAppError()

// 3) With Gin response helpers
// response.JSONError(ctx, appErr)
```

## What you get

- Stable string code and human message
- HTTP status
- Details map and field-level suggestions (great for validation)
- Retryable flag
- IDs: error ID and correlation/request ID
- Cause wrapping (errors.Is/As via `Unwrap()`)
- Conversions: `ParseServiceError(error)` and `ToAppError()`

---

## ServiceError API

Constructor and helpers:

```go
// From canonical ErrorCode (see pkg/apperr)
se := serr.FromCode(apperr.ErrorCodeInvalidRequest)

// Or directly
se := serr.NewServiceError("not_found", "User not found", http.StatusNotFound)

// Convenience helpers
se := serr.Internal("unexpected failure")
se := serr.NotFound("user not found")
se := serr.Forbidden("not allowed")
se := serr.Unauthorized("please login")
se := serr.BadRequest("payload invalid")
```

Functional options on construction:

```go
se := serr.NewServiceError(
  "invalid_input", "Bad email", http.StatusUnprocessableEntity,
  serr.WithDetail("field", "email"),
  serr.WithSuggestion("email", "Provide a valid email address"),
  serr.WithRetryable(false),
  serr.WithCorrelation(requestID),
)
```

Fluent mutators after construction:

```go
se := serr.NotFound("user not found").
  WithDetail("user_id", id).
  WithSuggestion("user_id", "Ensure the user exists").
  WithStatus(http.StatusNotFound)
```

Error interface and cause:

```go
func (se *ServiceError) Error() string
func (se *ServiceError) Unwrap() error // enables errors.Is/As
```

Conversions:

```go
// From any error → ServiceError
se := serr.ParseServiceError(err)

// ServiceError → AppError (for API responses)
appErr := se.ToAppError()
```

---

## Implementation process (recommended)

1) In your handlers and services, generate `ServiceError`s using helpers or `FromCode`.

```go
func (h *UserHandler) Get(ctx *gin.Context) {
  id := ctx.Param("id")
  user, err := h.svc.GetUser(ctx, id)
  if err != nil {
    se := serr.ParseServiceError(err)           // normalize
    response.JSONError(ctx, se.ToAppError())    // serialize
    return
  }
  response.Success(ctx, user)
}
```

2) For validation, attach suggestions and details:

```go
se := serr.BadRequest("validation failed").
  WithSuggestion("email", "Provide a valid email").
  WithDetail("field", "email")
```

3) Propagate correlation/request IDs through your middleware and attach via `WithCorrelation(id)` when creating `ServiceError`s (or via options in `NewServiceError`).

4) When integrating with libraries that return plain `error`, wrap them early:

```go
if err := repo.Save(user); err != nil {
  return serr.Internal("database error", serr.WithCause(err))
}
```

---

## Using the stack-trace wrapper (`Error`)

`Error` is a small wrapper that records a stack trace on creation/wrap.

```go
import coreerr "github.com/milan604/core-lab/pkg/errors"

err := coreerr.Wrap(someErr, "failed to process request")
if e, ok := err.(*coreerr.Error); ok {
  fmt.Println(e.StackTrace())
}
```

Use `Error` when you need stack diagnostics. Use `ServiceError` for user-facing error shaping.

---

## Migration notes

- Old service error shapes can be converted by mapping their fields to `NewServiceError` or `FromCode` and adding details/suggestions via options or fluent setters.
- For API responses, prefer `se.ToAppError()` and `response.JSONError(ctx, appErr)` to keep a consistent envelope.

````
# errors

Advanced error wrapping with stack traces.

## Usage
```go
import "github.com/milan604/core-lab/pkg/errors"
err := errors.Wrap(someErr, "failed to process request")
if e, ok := err.(*errors.Error); ok {
  fmt.Println(e.StackTrace())
}
```
