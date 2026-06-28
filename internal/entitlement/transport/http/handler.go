package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/entitlement/domain"
	entuc "virtual-exam-api/internal/entitlement/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
	userrepo "virtual-exam-api/internal/user/repository"
)

type Handler struct {
	entitlements *entuc.UseCase
	audit        *audituc.Logger
	users        userrepo.Repository
}

func NewHandler(entitlements *entuc.UseCase, audit *audituc.Logger, users userrepo.Repository) *Handler {
	return &Handler{entitlements: entitlements, audit: audit, users: users}
}

func (h *Handler) RegisterRoutes(admin *echo.Group) {
	admin.GET("/users/:userId/entitlements", h.ListUserEntitlements)
	admin.POST("/users/:userId/entitlements/exam-set", h.GrantExamSet)
	admin.POST("/users/:userId/entitlements/premium", h.GrantPremium)
	admin.DELETE("/entitlements/:id", h.Revoke)
}

type grantExamSetRequest struct {
	ExamSetID string  `json:"exam_set_id"`
	ExpiresAt *string `json:"expires_at"`
	Notes     *string `json:"notes"`
}

type grantPremiumRequest struct {
	ExpiresAt string  `json:"expires_at"`
	Notes     *string `json:"notes"`
}

func (h *Handler) ListUserEntitlements(c echo.Context) error {
	userID, err := parseUUID(c.Param("userId"))
	if err != nil {
		return response.Error(c, err)
	}
	pq := pagination.ParsePagination(c)
	result, err := h.entitlements.ListUserEntitlements(c.Request().Context(), userID, pq.Page, pq.Limit)
	if err != nil {
		return response.Error(c, err)
	}
	items := entuc.ToEntitlementResponses(result.Items)
	list := pagination.NewList(items, result.Page, result.Limit, result.TotalItems)
	return response.JSON(c, 200, list)
}

func (h *Handler) GrantExamSet(c echo.Context) error {
	userID, err := parseUUID(c.Param("userId"))
	if err != nil {
		return response.Error(c, err)
	}
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	var req grantExamSetRequest
	if err := c.Bind(&req); err != nil || req.ExamSetID == "" {
		return response.Error(c, apperrors.ErrInvalidEntitlement)
	}
	examSetID, err := uuid.Parse(req.ExamSetID)
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidUUID)
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return response.Error(c, apperrors.ValidationError("รูปแบบวันหมดอายุไม่ถูกต้อง"))
		}
		expiresAt = &t
	}

	ent, err := h.entitlements.GrantExamSetAccess(c.Request().Context(), domain.GrantExamSetAccessInput{
		UserID:    userID,
		ExamSetID: examSetID,
		ExpiresAt: expiresAt,
		Notes:     req.Notes,
		GrantedBy: actorID,
		Source:    domain.SourceManual,
	})
	if err != nil {
		return response.Error(c, err)
	}

	h.logAudit(c, actorID, "entitlement.grant_exam_set", ent.ID, ent.RefName, nil, entuc.ToEntitlementResponse(*ent))
	return response.JSON(c, 201, entuc.ToEntitlementResponse(*ent))
}

func (h *Handler) GrantPremium(c echo.Context) error {
	userID, err := parseUUID(c.Param("userId"))
	if err != nil {
		return response.Error(c, err)
	}
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	var req grantPremiumRequest
	if err := c.Bind(&req); err != nil || req.ExpiresAt == "" {
		return response.Error(c, apperrors.ErrInvalidEntitlement)
	}
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		return response.Error(c, apperrors.ValidationError("รูปแบบวันหมดอายุไม่ถูกต้อง"))
	}

	ent, err := h.entitlements.GrantPremiumAccess(c.Request().Context(), domain.GrantPremiumAccessInput{
		UserID:    userID,
		ExpiresAt: expiresAt,
		Notes:     req.Notes,
		GrantedBy: actorID,
		Source:    domain.SourceManual,
	})
	if err != nil {
		return response.Error(c, err)
	}

	h.logAudit(c, actorID, "entitlement.grant_premium", ent.ID, nil, nil, entuc.ToEntitlementResponse(*ent))
	return response.JSON(c, 201, entuc.ToEntitlementResponse(*ent))
}

func (h *Handler) Revoke(c echo.Context) error {
	entitlementID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	ent, err := h.entitlements.RevokeEntitlement(c.Request().Context(), entitlementID, actorID)
	if err != nil {
		return response.Error(c, err)
	}

	h.logAudit(c, actorID, "entitlement.revoke", ent.ID, ent.RefName, entuc.ToEntitlementResponse(*ent), map[string]any{"is_active": false})
	return response.JSON(c, 200, entuc.ToEntitlementResponse(*ent))
}

func (h *Handler) logAudit(c echo.Context, actorID uuid.UUID, action string, resourceID uuid.UUID, resourceName *string, before, after any) {
	if h.audit == nil {
		return
	}
	email := ""
	if actor, err := h.users.FindByID(c.Request().Context(), actorID); err == nil && actor != nil {
		email = actor.Email
	}
	name := ""
	if resourceName != nil {
		name = *resourceName
	}
	rid := resourceID
	h.audit.Log(c.Request().Context(), audituc.LogInput{
		ActorUserID:  &actorID,
		ActorEmail:   email,
		Action:       action,
		ResourceType: "entitlement",
		ResourceID:   &rid,
		ResourceName: name,
		BeforeData:   before,
		AfterData:    after,
		IPAddress:    c.RealIP(),
		UserAgent:    c.Request().UserAgent(),
	})
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperrors.ErrInvalidUUID
	}
	return id, nil
}
