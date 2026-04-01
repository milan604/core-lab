package validator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	gvalidator "github.com/go-playground/validator/v10"
	"github.com/milan604/core-lab/pkg/apperr"
)

func TestRegisterValidationAppliesToGinBinding(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	v := New()
	const tag = "corelab_release_ready_validator"
	if err := v.RegisterValidation(tag, func(fl gvalidator.FieldLevel) bool {
		return fl.Field().String() == "cool@example.com"
	}); err != nil {
		t.Fatalf("RegisterValidation() error = %v", err)
	}

	type request struct {
		Email string `json:"email_address" binding:"required,corelab_release_ready_validator"`
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email_address":"nope@example.com"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	var (
		appErr   error
		panicked any
	)
	func() {
		defer func() {
			panicked = recover()
		}()
		_, appErr = BindJSON[request](v, ctx)
	}()

	if panicked != nil {
		t.Fatalf("BindJSON panicked after RegisterValidation: %v", panicked)
	}
	if appErr == nil {
		t.Fatal("expected validation error")
	}

	validationErr, ok := appErr.(*apperr.AppError)
	if !ok {
		t.Fatalf("expected *apperr.AppError, got %T", appErr)
	}
	if len(validationErr.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(validationErr.Suggestions))
	}
	if got := validationErr.Suggestions[0].Field; got != "email_address" {
		t.Fatalf("expected field %q, got %q", "email_address", got)
	}
	if got := validationErr.Suggestions[0].Message; !strings.Contains(got, tag) {
		t.Fatalf("expected message to mention %q, got %q", tag, got)
	}
}
