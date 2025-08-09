# validator — Binding + Validation Utilities

Thin wrapper around go-playground/validator integrated with Gin binding. Converts binding/validation errors into `*apperr.AppError` and provides ergonomic bind helpers.

## Features
- Tag-based field name resolution (json/form/uri)
- Register custom validations and tag-to-message builders
- Error parsing into `*apperr.AppError` with field suggestions
- Binding helpers for JSON, Query, URI, Header
- Combined helpers to reduce handler boilerplate

## Quick Start
```go
v := validator.New()

// Optional: custom validations
_ = v.RegisterValidation("is-cool", func(fl gvalidator.FieldLevel) bool { return true })

// Optional: custom messages per tag
v.RegisterTagError("required", apperr.ErrorCodeValidationFail, func(fe gvalidator.FieldError) string {
  return fmt.Sprintf("%s is required", fe.Field())
})

// In handlers
type Body struct { Email string `json:"email" validate:"required,email"` }
body, appErr := validator.BindJSON[Body](v, c)
if appErr != nil { return c.Error(appErr) }
```

## Helpers
- BindJSON[T]
- BindQuery[T]
- BindURI[T]
- BindHeader[T]
- Combined:
  - BindJSONAndQuery[Body, Query]
  - BindJSONAndURI[Body, URI]
  - BindQueryAndURI[Query, URI]
  - BindJSONAndHeader[Body, Header]
  - BindQueryAndHeader[Query, Header]
  - BindAll[Body, Query, URI]

## Error Translation
`ParseError` maps:
- validator.ValidationErrors -> `validation_failed` with suggestions
- json.UnmarshalTypeError -> `invalid_request`
- json.SyntaxError -> `invalid_request`
- time.ParseError -> `invalid_input`
- default -> `invalid_input`

## Tips
- Define struct tags (`json`, `form`, `uri`, `header`) to control names in messages.
- Prefer `RegisterTagError` to keep client messages friendly.
- Use combined helpers to keep handlers clean and uniform.

---
Private and proprietary. All rights reserved.
