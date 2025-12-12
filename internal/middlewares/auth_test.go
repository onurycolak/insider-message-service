package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/onurcolak/insider-message-service/pkg/response"
)

func newEchoContext(method, path string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestAPIKeyAuth_MissingServerKeyReturns500(t *testing.T) {
	mw := APIKeyAuth("") // server misconfigured

	// Next handler should never be reached
	c, rec := newEchoContext(http.MethodGet, "/test")
	handler := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var body response.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Success {
		t.Errorf("expected Success=false, got true")
	}
	if body.Error == "" {
		t.Errorf("expected error message, got empty string")
	}
}

func TestAPIKeyAuth_MissingClientKeyReturns401(t *testing.T) {
	const serverKey = "secret"
	mw := APIKeyAuth(serverKey)

	c, rec := newEchoContext(http.MethodGet, "/test")
	handler := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// No x-ins-auth-key header
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	var body response.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Success {
		t.Errorf("expected Success=false, got true")
	}
	if body.Error == "" {
		t.Errorf("expected error message, got empty string")
	}
}

func TestAPIKeyAuth_InvalidClientKeyReturns401(t *testing.T) {
	const serverKey = "secret"
	mw := APIKeyAuth(serverKey)

	c, rec := newEchoContext(http.MethodGet, "/test")
	c.Request().Header.Set(APIKeyHeader, "wrong-key")

	handler := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_ValidKeyPassesThrough(t *testing.T) {
	const serverKey = "secret"
	mw := APIKeyAuth(serverKey)

	c, rec := newEchoContext(http.MethodGet, "/test")
	c.Request().Header.Set(APIKeyHeader, serverKey)

	handlerCalled := false
	handler := mw(func(c echo.Context) error {
		handlerCalled = true
		return c.NoContent(http.StatusOK)
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !handlerCalled {
		t.Fatalf("expected next handler to be called")
	}
}
