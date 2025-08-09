# response — API Response Helpers

Small helpers to emit consistent API envelopes for success and error responses, integrating with `pkg/apperr`.

## Envelope
```json
{
  "success": true,
  "code": "success",
  "message": "OK",
  "data": {},
  "meta": {"page": 1}
}
```

## Usage
```go
import (
  "corelab/pkg/response"
  "corelab/pkg/apperr"
)

// Success
response.JSONSuccess(c, http.StatusCreated, payload, map[string]any{"trace": rid})
response.Success(c, payload) // shorthand 200 OK

// Error
response.JSONError(c, apperr.New(apperr.ErrorCodeForbidden))
response.Error(c, fmt.Errorf("unknown")) // wraps to internal

// Centralized error handling
// Use middleware ErrorHandlerMiddleware() to translate c.Errors to JSON
```

## API
- `JSONSuccess(ctx, status, data, meta)`
- `JSONError(ctx, appErr)` — appErr is `*apperr.AppError` (wrap with `apperr.FromError`)
- `HandleError(ctx, err)` — accepts `error` and chooses the right envelope
- Shorthands: `Success(ctx, data)`, `Error(ctx, err)`

## Patterns
- Include `meta` for pagination, cursors, or request IDs.
- Keep error payloads small; log details with your logger.

---
Private and proprietary. All rights reserved.
