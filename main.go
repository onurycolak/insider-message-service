package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/handlers"
	"github.com/onurcolak/insider-message-service/internal/repository"
	"github.com/onurcolak/insider-message-service/internal/scheduler"
	"github.com/onurcolak/insider-message-service/internal/service"
	"github.com/onurcolak/insider-message-service/pkg/database"
	"github.com/onurcolak/insider-message-service/pkg/logger"
	"github.com/onurcolak/insider-message-service/pkg/redis"
	"github.com/onurcolak/insider-message-service/pkg/validator"
	"github.com/onurcolak/insider-message-service/pkg/webhook"
	"github.com/onurcolak/insider-message-service/routes"

	_ "github.com/onurcolak/insider-message-service/docs" // swagger docs
)

// @title Insider Message Service API
// @version 1.0
// @description Automatic message sending system for Insider One
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email onur.colak@useinsider.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /

// @schemes http https
func main() {
	logger.Init()

	// Load config
	cfg := environments.Load()

	// Hard-fail if required secrets are missing
	if cfg.Webhook.AuthKey == "" {
		logger.Fatalf("WEBHOOK_AUTH_KEY is required but not set")
	}
	if cfg.Auth.MessagesAPIKey == "" {
		logger.Fatalf("MESSAGES_API_KEY is required but not set")
	}
	if cfg.Auth.SchedulerAPIKey == "" {
		logger.Fatalf("SCHEDULER_API_KEY is required but not set")
	}

	logger.Infof("Starting Insider Message Service...")

	// Init DB
	db, err := database.NewMySQLDB(cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed data
	if os.Getenv("SEED_DATA") == "true" {
		if err := database.SeedTestData(db); err != nil {
			logger.Warnf("Failed to seed test data: %v", err)
		}
	}

	// Init redis
	var redisClient *redis.Client
	redisClient, err = redis.NewRedisClient(cfg.Redis)
	if err != nil {
		logger.Warnf("Redis not available, caching disabled: %v", err)
		redisClient = nil
	}

	// Initialize webhook client
	webhookClient := webhook.NewWebhookClient(cfg.Webhook)
	logger.Infof("Webhook configured: %s", webhookClient.GetURL())

	// Initialize repository
	messageRepo := repository.NewMessageRepository(db)

	// Initialize service
	messageService := service.NewMessageService(
		messageRepo,
		webhookClient,
		redisClient,
		cfg.Message,
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize scheduler
	sched := scheduler.NewScheduler(messageService, cfg.Message.SendInterval)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, redisClient)
	messageHandler := handlers.NewMessageHandler(messageService)
	schedulerHandler := handlers.NewSchedulerHandler(sched, ctx, cfg)

	// Auto-start scheduler
	if os.Getenv("AUTO_START_SCHEDULER") != "false" {
		logger.Infof("Auto-starting scheduler...")
		if err := sched.Start(ctx); err != nil {
			logger.Warnf("Failed to auto-start scheduler: %v", err)
		}
	}

	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			"x-ins-auth-key",
		},
	}))

	// Setup routes
	routes.RegisterRoutes(e, healthHandler, messageHandler, schedulerHandler, cfg)

	// Start server in goroutine
	go func() {
		addr := ":" + cfg.Server.Port
		logger.Infof("Server starting on http://localhost%s", addr)
		logger.Infof("Swagger docs available at http://localhost%s/swagger/index.html", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Infof("Shutting down gracefully...")

	// Cancel context to signal all goroutines to stop
	cancel()

	// Stop scheduler first (with timeout)
	if sched.IsRunning() {
		logger.Infof("Stopping scheduler...")
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()

		done := make(chan error, 1)
		go func() {
			done <- sched.Stop()
		}()

		select {
		case err := <-done:
			if err != nil {
				logger.Errorf("Error stopping scheduler: %v", err)
			} else {
				logger.Infof("Scheduler stopped successfully")
			}
		case <-stopCtx.Done():
			logger.Warnf("Scheduler stop timeout, forcing shutdown")
		}
	}

	// Shutdown HTTP server (with timeout)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logger.Infof("Shutting down HTTP server...")
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	} else {
		logger.Infof("HTTP server stopped successfully")
	}

	// Close database connection
	logger.Infof("Closing database connection...")
	if err := db.Close(); err != nil {
		logger.Errorf("Error closing database: %v", err)
	}

	// Close Redis connection
	if redisClient != nil {
		logger.Infof("Closing Redis connection...")
		if err := redisClient.Close(); err != nil {
			logger.Errorf("Error closing Redis: %v", err)
		}
	}

	logger.Infof("Graceful shutdown completed")
}
