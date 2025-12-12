package middlewares

import (
	"crypto/subtle"
	"fmt"

	"github.com/labstack/echo/v4"

	"github.com/onurcolak/insider-message-service/pkg/response"
)

const (
	APIKeyHeader = "x-ins-auth-key"
)

// secureCompare compares two strings in a way that is safer against timing attacks.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func APIKeyAuth(apiKey string) echo.MiddlewareFunc {
	// If the API key is not configured, treat this as a server-side misconfiguration.
	if apiKey == "" {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return response.InternalServerError(
					c,
					fmt.Errorf("API key is not configured for this endpoint group"),
				)
			}
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get API key from x-ins-auth-key header.
			token := c.Request().Header.Get(APIKeyHeader)
			if token == "" || !secureCompare(token, apiKey) {
				return response.Unauthorized(c)
			}

			return next(c)
		}
	}
}
