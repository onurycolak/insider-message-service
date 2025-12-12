package validator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type sampleRequest struct {
	Name  string `json:"name" validate:"required"`
	Phone string `json:"phone" validate:"required"`
}

func TestCustomValidator_ValidateReturnsValidationError(t *testing.T) {
	cv := New()

	req := sampleRequest{
		// Name and Phone left empty to trigger validation errors
	}

	err := cv.Validate(req)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}

	if len(ve.Errors) == 0 {
		t.Fatalf("expected at least one validation error, got none")
	}

	if _, exists := ve.Errors["name"]; !exists {
		t.Errorf("expected 'name' to be in validation errors")
	}
	if _, exists := ve.Errors["phone"]; !exists {
		t.Errorf("expected 'phone' to be in validation errors")
	}
}

func TestHandleValidationError_Returns422WithDetails(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	c := e.NewContext(req, rec)

	cv := New()
	err := cv.Validate(sampleRequest{})

	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	if err := HandleValidationError(c, err); err != nil {
		t.Fatalf("HandleValidationError returned error: %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", rec.Code)
	}

	var body ValidationErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Success {
		t.Errorf("expected Success=false, got true")
	}
	if body.Error != "Validation failed" {
		t.Errorf("expected error='Validation failed', got %q", body.Error)
	}
	if len(body.Details) == 0 {
		t.Fatalf("expected details in validation response, got none")
	}
}
