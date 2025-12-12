package routes

import (
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/handlers"
	"github.com/onurcolak/insider-message-service/internal/middlewares"
)

// RegisterRoutes registers all API routes with middleware
func RegisterRoutes(
	e *echo.Echo,
	healthHandler *handlers.HealthHandler,
	messageHandler *handlers.MessageHandler,
	schedulerHandler *handlers.SchedulerHandler,
	cfg *environments.Config,
) {
	e.GET("/health", healthHandler.Health)
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	// API v1 base group
	v1 := e.Group("/api/v1")

	// Message routes with their own API key
	messages := v1.Group("/messages", middlewares.APIKeyAuth(cfg.Auth.MessagesAPIKey))

	messages.GET("", messageHandler.GetAllMessages)
	messages.POST("", messageHandler.CreateMessage)
	messages.GET("/sent", messageHandler.GetSentMessages)
	messages.GET("/stats", messageHandler.GetStats)
	messages.GET("/cached", messageHandler.GetCachedMessages)

	// new replay endpoints
	messages.POST("/replay", messageHandler.ReplayAllFailedMessages)
	messages.POST("/:id/replay", messageHandler.ReplayFailedMessage)

	// Scheduler routes with their own API key
	schedulerGroup := v1.Group("/scheduler", middlewares.APIKeyAuth(cfg.Auth.SchedulerAPIKey))

	schedulerGroup.POST("/start", schedulerHandler.StartScheduler)
	schedulerGroup.POST("/stop", schedulerHandler.StopScheduler)
	schedulerGroup.GET("/status", schedulerHandler.GetSchedulerStatus)
}
