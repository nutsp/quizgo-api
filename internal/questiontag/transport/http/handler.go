package http

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
	tagrepo "virtual-exam-api/internal/questiontag/repository"
	taguc "virtual-exam-api/internal/questiontag/usecase"
	userrepo "virtual-exam-api/internal/user/repository"
)

type Handler struct {
	tags  *taguc.TagUseCase
	audit *audituc.Logger
	users userrepo.Repository
}

func NewHandler(tags *taguc.TagUseCase, audit *audituc.Logger, users userrepo.Repository) *Handler {
	return &Handler{tags: tags, audit: audit, users: users}
}

func (h *Handler) RegisterRoutes(admin *echo.Group) {
	admin.GET("/question-tags", h.List)
	admin.POST("/question-tags", h.Create)
	admin.GET("/question-tags/:id", h.Get)
	admin.PUT("/question-tags/:id", h.Update)
	admin.DELETE("/question-tags/:id", h.Delete)
}

func (h *Handler) List(c echo.Context) error {
	pq := pagination.ParsePagination(c)
	filter := tagrepo.TagAdminFilter{
		Query: pq.Q,
		Page:  pq.Page,
		Limit: pq.Limit,
		Sort:  pq.Sort,
		Order: pq.Order,
	}
	if v := c.QueryParam("is_active"); v != "" {
		active := v == "true"
		filter.IsActive = &active
	}
	result, err := h.tags.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Get(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.tags.Get(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Create(c echo.Context) error {
	var input taguc.TagInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.tags.Create(c.Request().Context(), input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) Update(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	before, _ := h.tags.Get(c.Request().Context(), id)
	var input taguc.TagInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.tags.Update(c.Request().Context(), id, input)
	if err != nil {
		return response.Error(c, err)
	}
	h.logAudit(c, "question_tag.update", "question_tag", &id, result.Name, before, result)
	return response.JSON(c, 200, result)
}

func (h *Handler) Delete(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.tags.Delete(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	if result == nil {
		return response.JSON(c, 200, map[string]string{"status": "deleted"})
	}
	return response.JSON(c, 200, map[string]any{
		"status":    "deactivated",
		"tag":       result,
		"message":   "กลุ่มคำถามนี้ถูกใช้งานอยู่ ระบบปิดใช้งานแทนการลบ",
	})
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperrors.ErrInvalidUUID
	}
	return id, nil
}

func queryInt(c echo.Context, key string) int {
	v, _ := strconv.Atoi(c.QueryParam(key))
	return v
}

func (h *Handler) logAudit(c echo.Context, action, resourceType string, resourceID *uuid.UUID, resourceName string, before, after any) {
	if h.audit == nil {
		return
	}
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return
	}
	email := ""
	if actor, err := h.users.FindByID(c.Request().Context(), actorID); err == nil && actor != nil {
		email = actor.Email
	}
	h.audit.Log(c.Request().Context(), audituc.LogInput{
		ActorUserID:  &actorID,
		ActorEmail:   email,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		BeforeData:   before,
		AfterData:    after,
		IPAddress:    c.RealIP(),
		UserAgent:    c.Request().UserAgent(),
	})
}
