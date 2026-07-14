package http

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	announcementrepo "virtual-exam-api/internal/announcement/repository"
	announcementuc "virtual-exam-api/internal/announcement/usecase"
	"virtual-exam-api/internal/apperrors"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
	userrepo "virtual-exam-api/internal/user/repository"
)

type Handler struct {
	announcements *announcementuc.UseCase
	audit         *audituc.Logger
	users         userrepo.Repository
}

func NewHandler(announcements *announcementuc.UseCase, audit *audituc.Logger, users userrepo.Repository) *Handler {
	return &Handler{announcements: announcements, audit: audit, users: users}
}

func (h *Handler) RegisterPublicRoutes(api *echo.Group) {
	api.GET("/announcements/active", h.ListActive)
	api.GET("/announcements/:slug", h.GetPublic)
	api.GET("/exam-tracks/:trackSlug/announcements", h.ListByTrack)
}

func (h *Handler) RegisterAdminRoutes(admin *echo.Group) {
	admin.GET("/announcements", h.ListAdmin)
	admin.POST("/announcements", h.Create)
	admin.GET("/announcements/:id", h.GetAdmin)
	admin.PATCH("/announcements/:id", h.Update)
	admin.PATCH("/announcements/:id/status", h.UpdateStatus)
	admin.DELETE("/announcements/:id", h.Delete)
}

func (h *Handler) ListAdmin(c echo.Context) error {
	pq := pagination.ParsePagination(c)
	filter := announcementrepo.AdminFilter{
		Query: c.QueryParam("q"), Type: c.QueryParam("type"),
		PublishStatus: c.QueryParam("publish_status"), Page: pq.Page, Limit: pq.Limit,
		Sort: pq.Sort, Order: pq.Order,
	}
	if value := c.QueryParam("is_active"); value != "" {
		active, err := strconv.ParseBool(value)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
		filter.IsActive = &active
	}
	result, err := h.announcements.ListAdmin(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetAdmin(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.announcements.GetAdmin(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Create(c echo.Context) error {
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	var input announcementuc.MutationInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.announcements.Create(c.Request().Context(), input, actorID)
	if err != nil {
		return response.Error(c, err)
	}
	h.logAudit(c, "announcement.create", idPointer(result.ID), result.Title, nil, result)
	if result.PublishStatus == "published" {
		h.logAudit(c, "announcement.publish", idPointer(result.ID), result.Title, nil, result)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) Update(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	before, err := h.announcements.GetAdmin(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	var input announcementuc.MutationInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.announcements.Update(c.Request().Context(), id, input, actorID)
	if err != nil {
		return response.Error(c, err)
	}
	h.logAudit(c, "announcement.update", &id, result.Title, before, result)
	if before.PublishStatus != result.PublishStatus {
		h.logAudit(c, statusAuditAction(string(result.PublishStatus)), &id, result.Title, before, result)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) UpdateStatus(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	before, err := h.announcements.GetAdmin(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	var input announcementuc.StatusInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.announcements.UpdateStatus(c.Request().Context(), id, input, actorID)
	if err != nil {
		return response.Error(c, err)
	}
	action := statusAuditAction(string(result.PublishStatus))
	h.logAudit(c, action, &id, result.Title, before, result)
	return response.JSON(c, 200, result)
}

func (h *Handler) Delete(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	deleted, err := h.announcements.Delete(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	h.logAudit(c, "announcement.delete", &id, deleted.Title, deleted, nil)
	return response.JSON(c, 200, map[string]bool{"deleted": true})
}

func (h *Handler) ListActive(c echo.Context) error {
	result, err := h.announcements.ListActive(c.Request().Context(), c.QueryParam("type"))
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetPublic(c echo.Context) error {
	result, err := h.announcements.GetPublic(c.Request().Context(), c.Param("slug"))
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ListByTrack(c echo.Context) error {
	result, err := h.announcements.ListByTrack(c.Request().Context(), c.Param("trackSlug"))
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) logAudit(c echo.Context, action string, resourceID *uuid.UUID, resourceName string, before, after any) {
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
		ActorUserID: &actorID, ActorEmail: email, Action: action,
		ResourceType: "announcement", ResourceID: resourceID, ResourceName: resourceName,
		BeforeData: before, AfterData: after, IPAddress: c.RealIP(), UserAgent: c.Request().UserAgent(),
	})
}

func parseUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, apperrors.ErrInvalidUUID
	}
	return id, nil
}

func idPointer(value string) *uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &id
}

func statusAuditAction(status string) string {
	switch status {
	case "published":
		return "announcement.publish"
	case "archived":
		return "announcement.archive"
	default:
		return "announcement.unpublish"
	}
}
