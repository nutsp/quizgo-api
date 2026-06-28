package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/accesslog/repository"
	accessuc "virtual-exam-api/internal/accesslog/usecase"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	admin *accessuc.AdminUseCase
}

func NewHandler(admin *accessuc.AdminUseCase) *Handler {
	return &Handler{admin: admin}
}

func (h *Handler) RegisterRoutes(admin *echo.Group) {
	admin.GET("/access-logs", h.List)
}

func (h *Handler) List(c echo.Context) error {
	pq := pagination.ParsePagination(c)
	filter := repository.AccessLogFilter{
		Email:     c.QueryParam("email"),
		EventType: c.QueryParam("event_type"),
		Page:      pq.Page,
		Limit:     pq.Limit,
		Sort:      pq.Sort,
		Order:     pq.Order,
	}
	if v := c.QueryParam("user_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidUUID)
		}
		filter.UserID = &id
	}
	if v := c.QueryParam("success"); v != "" {
		success := v == "true"
		filter.Success = &success
	}
	if v := c.QueryParam("date_from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
		filter.DateFrom = &t
	}
	if v := c.QueryParam("date_to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
		filter.DateTo = &t
	}
	result, err := h.admin.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}
