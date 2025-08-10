package response

import (
	"net/http"

	"github.com/milan604/core-lab/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// APIResponse is the standard API envelope returned to clients.
type APIResponse struct {
	Success bool                   `json:"success"`
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Data    interface{}            `json:"data,omitempty"`
	Errors  []apperr.Suggestion    `json:"errors,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// JSONSuccess writes a success envelope
func JSONSuccess(ctx *gin.Context, status int, data interface{}, meta map[string]interface{}) {
	if status == 0 {
		status = http.StatusOK
	}
	resp := APIResponse{
		Success: true,
		Code:    apperr.ErrorCodeSuccess.Code(),
		Message: apperr.ErrorCodeSuccess.Message(),
		Data:    data,
		Meta:    meta,
	}
	ctx.JSON(status, resp)
}

// JSONError writes an error envelope using *apperr.AppError
func JSONError(ctx *gin.Context, appErr *apperr.AppError) {
	if appErr == nil {
		appErr = apperr.New(apperr.ErrorCodeInternal)
	}
	status := appErr.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}
	resp := APIResponse{
		Success: false,
		Code:    appErr.Code,
		Message: appErr.Message,
		Errors:  appErr.Suggestions,
	}
	ctx.JSON(status, resp)
}

// HandleError is a convenience to accept generic error and return JSON error
func HandleError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	if ae, ok := err.(*apperr.AppError); ok {
		JSONError(ctx, ae)
		return
	}
	// fallback: wrap unknown errors
	JSONError(ctx, apperr.FromError(err))
}

// Success is a shorthand for JSONSuccess with http.StatusOK and no meta.
func Success(ctx *gin.Context, data interface{}) {
	JSONSuccess(ctx, http.StatusOK, data, nil)
}

// Error is a shorthand for JSONError from a plain error by converting to AppError.
func Error(ctx *gin.Context, err error) {
	if err == nil {
		JSONError(ctx, nil)
		return
	}
	if ae, ok := err.(*apperr.AppError); ok {
		JSONError(ctx, ae)
		return
	}
	JSONError(ctx, apperr.FromError(err))
}
