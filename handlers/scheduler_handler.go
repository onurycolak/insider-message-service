package handlers

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/internal/scheduler"
	"github.com/onurcolak/insider-message-service/pkg/response"
	"github.com/onurcolak/insider-message-service/pkg/validator"
)

type SchedulerHandler struct {
	scheduler *scheduler.Scheduler
	ctx       context.Context
	config    *environments.Config
}

type StartSchedulerRequest struct {
	Interval    *int     `json:"interval,omitempty" validate:"omitempty,min=1"`
	FailureRate *float64 `json:"failureRate,omitempty" validate:"omitempty,min=0,max=1"`
}

func NewSchedulerHandler(
	sched *scheduler.Scheduler,
	ctx context.Context,
	cfg *environments.Config,
) *SchedulerHandler {
	return &SchedulerHandler{
		scheduler: sched,
		ctx:       ctx,
		config:    cfg,
	}
}

// StartScheduler godoc
// @Summary Start the message scheduler
// @Description Starts the automatic message sending process with optional parameters
// @Tags scheduler
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for scheduler"
// @Param request body StartSchedulerRequest false "Scheduler parameters (optional)"
// @Success 200 {object} response.SuccessResponse
// @Failure 422 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/scheduler/start [post]
func (h *SchedulerHandler) StartScheduler(c echo.Context) error {
	if h.scheduler.IsRunning() {
		return response.OkWithMessage(c, "Scheduler is already running", h.scheduler.GetStatus())
	}

	var req StartSchedulerRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, err)
	}

	if err := c.Validate(&req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	// Default interval from configuration (e.g. 2 minutes per spec).
	intervalMinutes := int(h.config.Message.SendInterval.Minutes())
	if intervalMinutes <= 0 {
		intervalMinutes = 2
	}
	if req.Interval != nil {
		intervalMinutes = *req.Interval
	}

	failureRate := 0.0
	if req.FailureRate != nil {
		failureRate = *req.FailureRate
	}

	alertWebhook := h.config.Alert.WebhookURL
	alertThreshold := h.config.Alert.IterationCount

	if err := h.scheduler.StartWithParams(
		h.ctx,
		intervalMinutes,
		failureRate,
		alertWebhook,
		alertThreshold,
	); err != nil {
		return response.InternalServerError(c, err)
	}

	return response.OkWithMessage(c, "Scheduler started successfully", h.scheduler.GetStatus())
}

// StopScheduler godoc
// @Summary Stop the message scheduler
// @Description Stops the automatic message sending process
// @Tags scheduler
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for scheduler"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/scheduler/stop [post]
func (h *SchedulerHandler) StopScheduler(c echo.Context) error {
	if !h.scheduler.IsRunning() {
		return response.OkWithMessage(c, "Scheduler is already stopped", h.scheduler.GetStatus())
	}

	if err := h.scheduler.Stop(); err != nil {
		return response.InternalServerError(c, err)
	}

	return response.OkWithMessage(c, "Scheduler stopped successfully", h.scheduler.GetStatus())
}

// GetSchedulerStatus godoc
// @Summary Get scheduler status
// @Description Returns the current status of the message scheduler
// @Tags scheduler
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for scheduler"
// @Success 200 {object} response.SuccessResponse
// @Router /api/v1/scheduler/status [get]
func (h *SchedulerHandler) GetSchedulerStatus(c echo.Context) error {
	return response.Ok(c, h.scheduler.GetStatus())
}
