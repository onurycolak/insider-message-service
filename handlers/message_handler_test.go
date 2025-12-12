package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/onurcolak/insider-message-service/pkg/response"
	validatorpkg "github.com/onurcolak/insider-message-service/pkg/validator"
)

// TestCreateMessage_BadJSON verifies that invalid JSON returns 400 Bad Request.
func TestCreateMessage_BadJSON(t *testing.T) {
	e := echo.New()
	// Validator is not needed here because Bind will fail before Validate is called.
	handler := NewMessageHandler(nil)

	// Malformed JSON (missing closing quote / brace)
	reqBody := `{"content": "Hello", "phoneNumber":`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateMessage(c)
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp response.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	if resp.Success {
		t.Fatalf("expected Success=false, got true")
	}
	if resp.Error == "" {
		t.Fatalf("expected Error to be non-empty")
	}
}

// TestCreateMessage_TooLongContent verifies that validation failure (content > max)
// returns 422 Unprocessable Entity via the validation error handler.
func TestCreateMessage_TooLongContent(t *testing.T) {
	e := echo.New()
	// Use the real custom validator so we exercise the normal flow.
	e.Validator = validatorpkg.New()

	// service is nil on purpose; we want validation to fail before service is called.
	handler := NewMessageHandler(nil)

	// Content longer than the 1000-char max in the struct tag.
	longContent := strings.Repeat("a", 1001)
	reqBody := `{"content": "` + longContent + `", "phoneNumber": "+905551234567"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateMessage(c)
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, rec.Code)
	}

	var resp validatorpkg.ValidationErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	if resp.Success {
		t.Fatalf("expected Success=false, got true")
	}
	if resp.Error != "Validation failed" {
		t.Fatalf("expected Error=%q, got %q", "Validation failed", resp.Error)
	}
	if len(resp.Details) == 0 {
		t.Fatalf("expected Details to contain at least one field error")
	}
	if _, ok := resp.Details["content"]; !ok {
		t.Fatalf("expected Details to contain 'content' key")
	}
}
