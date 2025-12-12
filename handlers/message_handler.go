package handlers

import (
	"fmt"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/onurcolak/insider-message-service/internal/domain"
	"github.com/onurcolak/insider-message-service/internal/service"
	"github.com/onurcolak/insider-message-service/pkg/response"
	"github.com/onurcolak/insider-message-service/pkg/validator"
)

type MessageHandler struct {
	service *service.MessageService
}

func NewMessageHandler(service *service.MessageService) *MessageHandler {
	return &MessageHandler{service: service}
}

type CreateMessageRequest struct {
	Content     string `json:"content" validate:"required,max=1000"`
	PhoneNumber string `json:"phoneNumber" validate:"required"`
}

// GetSentMessages godoc
// @Summary Get sent messages
// @Description Retrieves a paginated list of all sent messages
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Param page query int false "Page number (default: 1)"
// @Param pageSize query int false "Page size (default: 20, max: 100)"
// @Success 200 {object} response.PaginatedResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages/sent [get]
func (h *MessageHandler) GetSentMessages(c echo.Context) error {
	page, pageSize, err := parsePaginationParams(c)
	if err != nil {
		return response.BadRequest(c, err)
	}

	messages, totalCount, err := h.service.GetSentMessages(c.Request().Context(), page, pageSize)
	if err != nil {
		return response.InternalServerError(c, err)
	}

	return response.Paginated(c, messages, page, pageSize, totalCount)
}

// GetAllMessages godoc
// @Summary Get all messages
// @Description Retrieves a paginated list of all messages with optional status filter
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Param page query int false "Page number (default: 1)"
// @Param pageSize query int false "Page size (default: 20, max: 100)"
// @Param status query string false "Filter by status (pending, sent, failed)"
// @Success 200 {object} response.PaginatedResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages [get]
func (h *MessageHandler) GetAllMessages(c echo.Context) error {
	page, pageSize, err := parsePaginationParams(c)
	if err != nil {
		return response.BadRequest(c, err)
	}

	statusStr := c.QueryParam("status")

	// Convert status string to pointer (optional filter).
	var status *domain.MessageStatus
	if statusStr != "" {
		parsedStatus := domain.MessageStatus(statusStr)
		status = &parsedStatus
	}

	messages, totalCount, err := h.service.GetAllMessages(c.Request().Context(), status, page, pageSize)
	if err != nil {
		return response.InternalServerError(c, err)
	}

	return response.Paginated(c, messages, page, pageSize, totalCount)
}

// CreateMessage godoc
// @Summary Create a new message
// @Description Creates a new message to be sent by the scheduler
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Param message body CreateMessageRequest true "Message to create"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages [post]
func (h *MessageHandler) CreateMessage(c echo.Context) error {
	var req CreateMessageRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, err)
	}

	if err := c.Validate(&req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	message, err := h.service.CreateMessage(c.Request().Context(), req.Content, req.PhoneNumber)
	if err != nil {
		return response.InternalServerError(c, err)
	}

	return response.Created(c, "Message created successfully", message)
}

// GetStats godoc
// @Summary Get message statistics
// @Description Returns count of messages by status
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages/stats [get]
func (h *MessageHandler) GetStats(c echo.Context) error {
	pending, sent, failed, err := h.service.GetStats(c.Request().Context())
	if err != nil {
		return response.InternalServerError(c, err)
	}

	return response.Ok(c, map[string]any{
		"pending": pending,
		"sent":    sent,
		"failed":  failed,
		"total":   pending + sent + failed,
	})
}

// GetCachedMessages godoc
// @Summary Get cached messages from Redis
// @Description Returns all messages cached in Redis (bonus feature)
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages/cached [get]
func (h *MessageHandler) GetCachedMessages(c echo.Context) error {
	cached, err := h.service.GetCachedMessages(c.Request().Context())
	if err != nil {
		return response.InternalServerError(c, err)
	}

	return response.Ok(c, cached)
}

func parsePaginationParams(c echo.Context) (int, int, error) {
	const (
		defaultPage     = 1
		defaultPageSize = 20
		maxPageSize     = 100
	)

	pageStr := c.QueryParam("page")
	pageSizeStr := c.QueryParam("pageSize")

	// Page
	page := defaultPage
	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p <= 0 {
			return 0, 0, fmt.Errorf("page must be a positive integer")
		}
		page = p
	}

	// Page size
	pageSize := defaultPageSize
	if pageSizeStr != "" {
		ps, err := strconv.Atoi(pageSizeStr)
		if err != nil || ps <= 0 || ps > maxPageSize {
			return 0, 0, fmt.Errorf("pageSize must be between 1 and %d", maxPageSize)
		}

		pageSize = ps
	}

	return page, pageSize, nil
}

// ReplayAllFailedMessages godoc
// @Summary Replay all failed messages
// @Description Sets status='pending' for all failed messages so the scheduler can resend them
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages/replay [post]
func (h *MessageHandler) ReplayAllFailedMessages(c echo.Context) error {
	count, err := h.service.ReplayAllFailedMessages(c.Request().Context())
	if err != nil {
		return response.InternalServerError(c, err)
	}

	return response.Ok(c, map[string]any{
		"replayed": count,
	})
}

// ReplayFailedMessage godoc
// @Summary Replay a single failed message
// @Description Sets status='pending' for a specific failed message so the scheduler can resend it
// @Tags messages
// @Accept json
// @Produce json
// @Param x-ins-auth-key header string true "API key for messages"
// @Param id path int true "Message ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/messages/{id}/replay [post]
func (h *MessageHandler) ReplayFailedMessage(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return response.BadRequest(c, fmt.Errorf("invalid message id"))
	}

	if err := h.service.ReplayFailedMessage(c.Request().Context(), id); err != nil {
		// We treat "no failed message found" as a 400 here to avoid adding a new NotFound helper.
		return response.BadRequest(c, err)
	}

	return response.Ok(c, map[string]any{
		"replayed": 1,
	})
}
