package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type PaginatedResponse struct {
	Success    bool  `json:"success"`
	Data       any   `json:"data"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalCount int64 `json:"totalCount"`
	TotalPages int   `json:"totalPages"`
}

func Ok(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Data:    data,
	})
}

func OkWithMessage(c echo.Context, message string, data any) error {
	return c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Created(c echo.Context, message string, data any) error {
	return c.JSON(http.StatusCreated, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func BadRequest(c echo.Context, err error) error {
	return c.JSON(http.StatusBadRequest, ErrorResponse{
		Success: false,
		Error:   err.Error(),
	})
}

func BadRequestWithMessage(c echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, ErrorResponse{
		Success: false,
		Error:   message,
	})
}

func Unauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, ErrorResponse{
		Success: false,
		Error:   "Invalid or missing API key",
	})
}

func NotFound(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, ErrorResponse{
		Success: false,
		Error:   message,
	})
}

func InternalServerError(c echo.Context, err error) error {
	return c.JSON(http.StatusInternalServerError, ErrorResponse{
		Success: false,
		Error:   err.Error(),
	})
}

func UnprocessableEntity(c echo.Context, err error) error {
	return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{
		Success: false,
		Error:   err.Error(),
	})
}

func Paginated(c echo.Context, data any, page, pageSize int, totalCount int64) error {
	totalPages := int(totalCount) / pageSize
	if int(totalCount)%pageSize > 0 {
		totalPages++
	}

	return c.JSON(http.StatusOK, PaginatedResponse{
		Success:    true,
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	})
}
