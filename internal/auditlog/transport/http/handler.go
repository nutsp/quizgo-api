package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	auditrepo "virtual-exam-api/internal/auditlog/repository"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	admin *audituc.AdminUseCase
}

func NewHandler(admin *audituc.AdminUseCase) *Handler {
	return &Handler{admin: admin}
}

func (h *Handler) RegisterRoutes(admin *echo.Group) {
	admin.GET("/audit-logs", h.List)
	admin.GET("/audit-logs/:id", h.Get)
}

func (h *Handler) List(c echo.Context) error {
	pq := pagination.ParsePagination(c)
	filter := auditrepo.AuditLogFilter{
		Action:       c.QueryParam("action"),
		ResourceType: c.QueryParam("resource_type"),
		Page:         pq.Page,
		Limit:        pq.Limit,
		Sort:         pq.Sort,
		Order:        pq.Order,
	}
	if v := c.QueryParam("actor_user_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidUUID)
		}
		filter.ActorUserID = &id
	}
	if v := c.QueryParam("resource_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidUUID)
		}
		filter.ResourceID = &id
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

func (h *Handler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidUUID)
	}
	result, err := h.admin.Get(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	if result == nil {
		return response.Error(c, apperrors.ErrNotFound)
	}
	return response.JSON(c, 200, result)
}
