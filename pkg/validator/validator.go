package validator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/milan604/core-lab/pkg/apperr"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	gvalidator "github.com/go-playground/validator/v10"
)

// FieldError represents a single field validation problem.
type FieldError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Tag     string      `json:"tag,omitempty"`
	Param   string      `json:"param,omitempty"`
	Value   interface{} `json:"value,omitempty"`
}

// TagErrorBuilder describes how to convert a validator.FieldError into a message
type TagErrorBuilder struct {
	Code    *apperr.ErrorCode
	Builder func(fe gvalidator.FieldError) string
}

// Validator is the wrapper around go-playground validator with extra features.
type Validator struct {
	v                *gvalidator.Validate
	tagErrorBuilders map[string]TagErrorBuilder
	fieldNameFn      func(reflect.StructField) string
}

// ValidatorEngine defines the interface for validation engines
// This allows for custom implementations and easier testing
// Example methods: RegisterValidation, RegisterTagError, ParseError, etc.
type ValidatorEngine interface {
	RegisterValidation(tag string, fn gvalidator.Func) error
	RegisterTagError(tag string, code *apperr.ErrorCode, builder func(gvalidator.FieldError) string)
	ParseError(err error) *apperr.AppError
}

// New creates a new Validator instance and wires up Gin's validator engine for tag->name resolution.
func New() *Validator {
	v := gvalidator.New()

	// set up default field name function similar to your earlier impl
	fieldNameFn := func(f reflect.StructField) string {
		if name := getTagName(f, "json"); name != "" {
			return name
		}
		if name := getTagName(f, "form"); name != "" {
			return name
		}
		if name := getTagName(f, "uri"); name != "" {
			return name
		}
		return f.Name
	}

	// register with gin binding engine so FieldError.Field() reflects tag name
	if be, ok := binding.Validator.Engine().(*gvalidator.Validate); ok {
		be.RegisterTagNameFunc(func(fld reflect.StructField) string {
			return fieldNameFn(fld)
		})
		// copy other settings if needed
	}

	return &Validator{
		v:                v,
		tagErrorBuilders: make(map[string]TagErrorBuilder),
		fieldNameFn:      fieldNameFn,
	}
}

// helper to get tag name
func getTagName(f reflect.StructField, tagName string) string {
	tagValue := f.Tag.Get(tagName)
	if tagValue == "-" {
		return ""
	}
	return strings.SplitN(tagValue, ",", 2)[0]
}

// RegisterValidation registers a custom validator (name) to the engine.
func (vi *Validator) RegisterValidation(tag string, fn gvalidator.Func) error {
	return vi.v.RegisterValidation(tag, fn)
}

// RegisterTagError allows mapping tag -> ErrorCode + message builder.
func (vi *Validator) RegisterTagError(tag string, code *apperr.ErrorCode, builder func(gvalidator.FieldError) string) {
	vi.tagErrorBuilders[tag] = TagErrorBuilder{Code: code, Builder: builder}
}

// ParseError converts any binding/validator/json error into *apperr.AppError
func (vi *Validator) ParseError(err error) *apperr.AppError {
	if err == nil {
		return nil
	}

	// validator errors
	switch e := err.(type) {
	case gvalidator.ValidationErrors:
		appErr := apperr.New(apperr.ErrorCodeValidationFail)
		for _, fe := range e {
			field := fe.Field() // thanks to registered TagNameFunc this will be the json/form name
			msg := vi.buildMessageForField(fe)
			appErr.AddSuggestion(field, msg)
		}
		return appErr

	case *json.UnmarshalTypeError:
		appErr := apperr.New(apperr.ErrorCodeInvalidRequest)
		f := e.Field
		if f == "" {
			// best-effort: sometimes field is empty for top-level decode errors
			appErr.Message = "Invalid request body"
			return appErr
		}
		msg := fmt.Sprintf("Invalid type for field %s: expected %s", f, e.Type.String())
		appErr.AddSuggestion(f, msg)
		return appErr

	case *json.SyntaxError:
		appErr := apperr.New(apperr.ErrorCodeInvalidRequest)
		appErr.Message = "Invalid JSON payload"
		return appErr

	case *time.ParseError:
		appErr := apperr.New(apperr.ErrorCodeInvalidInput)
		appErr.Message = "Invalid datetime format"
		return appErr

	default:
		// generic error -> return InvalidInput but keep underlying message in server logs
		appErr := apperr.New(apperr.ErrorCodeInvalidInput)
		appErr.Message = fmt.Sprintf("Invalid input: %v", err.Error())
		return appErr
	}
}

// buildMessageForField uses registered tag builders or defaults
func (vi *Validator) buildMessageForField(fe gvalidator.FieldError) string {
	if b, ok := vi.tagErrorBuilders[fe.Tag()]; ok && b.Builder != nil {
		return b.Builder(fe)
	}
	// default message
	if fe.Param() != "" {
		return fmt.Sprintf("field %s failed on '%s' validation (param=%s)", fe.Field(), fe.Tag(), fe.Param())
	}
	return fmt.Sprintf("field %s failed on '%s' validation", fe.Field(), fe.Tag())
}

/* ------------------------------
   Binding helpers (Gin friendly)
   These reduce boilerplate in handlers:
   - BindJSON
   - BindQuery
	- BindURI
	- BindHeader
	 - Combined helpers
		- BindJSONAndQuery
		- BindJSONAndURI
		- BindQueryAndURI
		- BindAll (JSON, Query, URI)
		- BindJSONAndHeader
		- BindQueryAndHeader
--------------------------------*/

// BindJSON binds and validates JSON body into T. Returns either (*T, nil) or (nil, *apperr.AppError)
func BindJSON[T any](vi ValidatorEngine, ctx *gin.Context) (*T, *apperr.AppError) {
	var req T
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, vi.ParseError(err)
	}
	return &req, nil
}

// BindQuery binds & validates query parameters
func BindQuery[T any](vi ValidatorEngine, ctx *gin.Context) (*T, *apperr.AppError) {
	var req T
	if err := ctx.ShouldBindQuery(&req); err != nil {
		return nil, vi.ParseError(err)
	}
	return &req, nil
}

// BindURI binds & validates uri params
func BindURI[T any](vi ValidatorEngine, ctx *gin.Context) (*T, *apperr.AppError) {
	var req T
	if err := ctx.ShouldBindUri(&req); err != nil {
		return nil, vi.ParseError(err)
	}
	return &req, nil
}

// BindHeader binds & validates header params
func BindHeader[T any](vi ValidatorEngine, ctx *gin.Context) (*T, *apperr.AppError) {
	var req T
	if err := ctx.ShouldBindHeader(&req); err != nil {
		return nil, vi.ParseError(err)
	}
	return &req, nil
}

// BindJSONAndURI binds and validates both JSON body and URI params
func BindJSONAndURI[Body any, URI any](vi ValidatorEngine, ctx *gin.Context) (*Body, *URI, *apperr.AppError) {
	body, be := BindJSON[Body](vi, ctx)
	if be != nil {
		return body, new(URI), be
	}
	uri, ue := BindURI[URI](vi, ctx)
	if ue != nil {
		return body, uri, ue
	}
	return body, uri, nil
}

// BindQueryAndURI binds and validates both query params and URI params
func BindQueryAndURI[Query any, URI any](vi ValidatorEngine, ctx *gin.Context) (*Query, *URI, *apperr.AppError) {
	query, qe := BindQuery[Query](vi, ctx)
	if qe != nil {
		return query, new(URI), qe
	}
	uri, ue := BindURI[URI](vi, ctx)
	if ue != nil {
		return query, uri, ue
	}
	return query, uri, nil
}

// BindJSONAndQuery binds and validates both JSON body and query parameters
func BindJSONAndQuery[Body any, Query any](vi ValidatorEngine, ctx *gin.Context) (*Body, *Query, *apperr.AppError) {
	body, be := BindJSON[Body](vi, ctx)
	if be != nil {
		return body, new(Query), be
	}
	query, qe := BindQuery[Query](vi, ctx)
	if qe != nil {
		return body, query, qe
	}
	return body, query, nil
}

// BindAll binds and validates JSON body, query params, and URI params
func BindAll[Body any, Query any, URI any](vi ValidatorEngine, ctx *gin.Context) (*Body, *Query, *URI, *apperr.AppError) {
	body, be := BindJSON[Body](vi, ctx)
	if be != nil {
		return body, new(Query), new(URI), be
	}
	query, qe := BindQuery[Query](vi, ctx)
	if qe != nil {
		return body, query, new(URI), qe
	}
	uri, ue := BindURI[URI](vi, ctx)
	if ue != nil {
		return body, query, uri, ue
	}
	return body, query, uri, nil
}

// BindJSONAndHeader binds JSON body and headers
func BindJSONAndHeader[Body any, Header any](vi ValidatorEngine, ctx *gin.Context) (*Body, *Header, *apperr.AppError) {
	body, be := BindJSON[Body](vi, ctx)
	if be != nil {
		return body, new(Header), be
	}
	header, he := BindHeader[Header](vi, ctx)
	if he != nil {
		return body, header, he
	}
	return body, header, nil
}

// BindQueryAndHeader binds query params and headers
func BindQueryAndHeader[Query any, Header any](vi ValidatorEngine, ctx *gin.Context) (*Query, *Header, *apperr.AppError) {
	query, qe := BindQuery[Query](vi, ctx)
	if qe != nil {
		return query, new(Header), qe
	}
	header, he := BindHeader[Header](vi, ctx)
	if he != nil {
		return query, header, he
	}
	return query, header, nil
}
