package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"

	"github.com/onurcolak/insider-message-service/pkg/redis"
)

// HealthHandler handles health checks.
type HealthHandler struct {
	db           *sqlx.DB
	redis        *redis.Client
	checkTimeout time.Duration
}

func NewHealthHandler(db *sqlx.DB, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:           db,
		redis:        redisClient,
		checkTimeout: 2 * time.Second,
	}
}

// Health returns overall status and basic component statuses (DB and Redis).
// @Summary Health check
// @Description Returns overall status with DB and Redis connectivity results
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /health [get]
func (h *HealthHandler) Health(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), h.checkTimeout)
	defer cancel()

	overallStatus := "ok"

	dbStatus := "up"
	if h.db == nil {
		dbStatus = "down"
		overallStatus = "down"
	} else if err := h.db.PingContext(ctx); err != nil {
		dbStatus = "down"
		overallStatus = "down"
	}

	redisStatus := "disabled"
	if h.redis != nil {
		if err := h.redis.Ping(ctx); err != nil {
			redisStatus = "down"
			overallStatus = "degraded"
		} else {
			redisStatus = "up"
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":    overallStatus,
		"timestamp": time.Now().Format(time.RFC3339),
		"components": map[string]any{
			"database": map[string]any{
				"status": dbStatus,
			},
			"redis": map[string]any{
				"status": redisStatus,
			},
		},
	})
}
