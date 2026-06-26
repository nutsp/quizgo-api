package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	accessdomain "virtual-exam-api/internal/accesslog/domain"
	accessrepo "virtual-exam-api/internal/accesslog/repository"
	auditdomain "virtual-exam-api/internal/auditlog/domain"
	auditrepo "virtual-exam-api/internal/auditlog/repository"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	entdomain "virtual-exam-api/internal/entitlement/domain"
	entrepo "virtual-exam-api/internal/entitlement/repository"
	userdomain "virtual-exam-api/internal/user/domain"
	useradminrepo "virtual-exam-api/internal/useradmin/repository"
)

type RequestContext struct {
	ActorUserID uuid.UUID
	ActorEmail  string
	IPAddress   string
	UserAgent   string
}

type UseCase struct {
	users        useradminrepo.UserAdminRepository
	entitlements entrepo.Repository
	accessLog    accessrepo.Repository
	auditLog     auditrepo.Repository
	audit        *audituc.Logger
}

func NewUseCase(
	users useradminrepo.UserAdminRepository,
	entitlements entrepo.Repository,
	accessLog accessrepo.Repository,
	auditLog auditrepo.Repository,
	audit *audituc.Logger,
) *UseCase {
	return &UseCase{
		users:        users,
		entitlements: entitlements,
		accessLog:    accessLog,
		auditLog:     auditLog,
		audit:        audit,
	}
}

type UserResponse struct {
	ID            string               `json:"id"`
	Email         string               `json:"email"`
	DisplayName   string               `json:"display_name"`
	Role          string               `json:"role"`
	Status        string               `json:"status"`
	LastLoginAt   *string              `json:"last_login_at,omitempty"`
	CreatedAt     string               `json:"created_at"`
	AccessSummary AccessSummaryResponse `json:"access_summary"`
}

type AccessSummaryResponse struct {
	DisplayAccessType  string  `json:"display_access_type"`
	HasPremium         bool    `json:"has_premium"`
	ActiveExamSetCount int     `json:"active_exam_set_count"`
	PremiumExpiresAt   *string `json:"premium_expires_at"`
}

type UserListResponse = pagination.PaginatedList[UserResponse]

type AccessLogSummary struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Success   bool   `json:"success"`
	IPAddress string `json:"ip_address,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at"`
}

type AuditLogSummary struct {
	ID           string `json:"id"`
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type DetailResponse struct {
	UserResponse
	RecentAccessLogs []AccessLogSummary `json:"recent_access_logs"`
	RecentAuditLogs  []AuditLogSummary  `json:"recent_audit_logs"`
}

type UpdateInput struct {
	DisplayName *string `json:"display_name"`
	Role        *string `json:"role"`
	Status      *string `json:"status"`
}

func (uc *UseCase) List(ctx context.Context, filter useradminrepo.UserAdminFilter) (*UserListResponse, error) {
	items, total, err := uc.users.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	userIDs := make([]uuid.UUID, len(items))
	for i, u := range items {
		userIDs[i] = u.ID
	}

	now := time.Now().UTC()
	summaries, err := uc.entitlements.SummarizeActiveByUserIDs(ctx, userIDs, now)
	if err != nil {
		return nil, err
	}

	resp := make([]UserResponse, len(items))
	for i, u := range items {
		summary := summaries[u.ID]
		resp[i] = toUserResponse(u, entdomain.BuildAccessSummary(
			summary.HasPremium,
			summary.ActiveExamSetCount,
			summary.PremiumExpiresAt,
		))
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func (uc *UseCase) Get(ctx context.Context, id uuid.UUID) (*DetailResponse, error) {
	user, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrNotFound
	}
	accessLogs, _ := uc.accessLog.ListRecentByUserID(ctx, id, 10)
	auditLogs, _ := uc.auditLog.ListRecentByResource(ctx, "user", id, 10)
	return &DetailResponse{
		UserResponse:     toUserResponse(*user, defaultAccessSummary(ctx, uc, id)),
		RecentAccessLogs: toAccessSummaries(accessLogs),
		RecentAuditLogs:  toAuditSummaries(auditLogs),
	}, nil
}

func (uc *UseCase) Update(ctx context.Context, id uuid.UUID, input UpdateInput, reqCtx RequestContext) (*UserResponse, error) {
	user, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrNotFound
	}
	before := userSnapshot(user)
	if input.DisplayName != nil {
		name := strings.TrimSpace(*input.DisplayName)
		if name == "" {
			return nil, apperrors.ValidationError("กรุณาระบุชื่อผู้ใช้งาน")
		}
		if err := uc.users.UpdateDisplayName(ctx, id, name); err != nil {
			return nil, err
		}
	}
	if input.Role != nil {
		if err := uc.validateRoleChange(ctx, user, reqCtx.ActorUserID, *input.Role); err != nil {
			return nil, err
		}
		if err := uc.users.UpdateRole(ctx, id, *input.Role); err != nil {
			return nil, err
		}
	}
	if input.Status != nil {
		if err := uc.validateStatusChange(ctx, user, reqCtx.ActorUserID, *input.Status); err != nil {
			return nil, err
		}
		if err := uc.users.UpdateStatus(ctx, id, *input.Status); err != nil {
			return nil, err
		}
	}
	updated, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	after := userSnapshot(updated)
	uc.audit.Log(ctx, audituc.LogInput{
		ActorUserID:  &reqCtx.ActorUserID,
		ActorEmail:   reqCtx.ActorEmail,
		Action:       "user.update",
		ResourceType: "user",
		ResourceID:   &id,
		ResourceName: updated.DisplayName,
		BeforeData:   before,
		AfterData:    after,
		IPAddress:    reqCtx.IPAddress,
		UserAgent:    reqCtx.UserAgent,
	})
	resp := toUserResponse(*updated, defaultAccessSummary(ctx, uc, id))
	return &resp, nil
}

func (uc *UseCase) UpdateStatus(ctx context.Context, id uuid.UUID, status string, reqCtx RequestContext) (*UserResponse, error) {
	user, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrNotFound
	}
	if err := validateStatus(status); err != nil {
		return nil, err
	}
	if err := uc.validateStatusChange(ctx, user, reqCtx.ActorUserID, status); err != nil {
		return nil, err
	}
	before := userSnapshot(user)
	if err := uc.users.UpdateStatus(ctx, id, status); err != nil {
		return nil, err
	}
	updated, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	after := userSnapshot(updated)
	uc.audit.Log(ctx, audituc.LogInput{
		ActorUserID:  &reqCtx.ActorUserID,
		ActorEmail:   reqCtx.ActorEmail,
		Action:       "user.status_update",
		ResourceType: "user",
		ResourceID:   &id,
		ResourceName: updated.DisplayName,
		BeforeData:   before,
		AfterData:    after,
		IPAddress:    reqCtx.IPAddress,
		UserAgent:    reqCtx.UserAgent,
	})
	resp := toUserResponse(*updated, defaultAccessSummary(ctx, uc, id))
	return &resp, nil
}

func (uc *UseCase) UpdateRole(ctx context.Context, id uuid.UUID, role string, reqCtx RequestContext) (*UserResponse, error) {
	user, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrNotFound
	}
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if err := uc.validateRoleChange(ctx, user, reqCtx.ActorUserID, role); err != nil {
		return nil, err
	}
	before := userSnapshot(user)
	if err := uc.users.UpdateRole(ctx, id, role); err != nil {
		return nil, err
	}
	updated, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	after := userSnapshot(updated)
	uc.audit.Log(ctx, audituc.LogInput{
		ActorUserID:  &reqCtx.ActorUserID,
		ActorEmail:   reqCtx.ActorEmail,
		Action:       "user.role_update",
		ResourceType: "user",
		ResourceID:   &id,
		ResourceName: updated.DisplayName,
		BeforeData:   before,
		AfterData:    after,
		IPAddress:    reqCtx.IPAddress,
		UserAgent:    reqCtx.UserAgent,
	})
	resp := toUserResponse(*updated, defaultAccessSummary(ctx, uc, id))
	return &resp, nil
}

func (uc *UseCase) validateRoleChange(ctx context.Context, user *userdomain.User, actorID uuid.UUID, newRole string) error {
	if err := validateRole(newRole); err != nil {
		return err
	}
	if user.Role == userdomain.RoleAdmin && newRole != userdomain.RoleAdmin {
		count, err := uc.users.CountActiveAdminsExcept(ctx, user.ID)
		if err != nil {
			return err
		}
		if count < 1 && user.Status == userdomain.StatusActive {
			return apperrors.ValidationError("ไม่สามารถเปลี่ยนบทบาทผู้ดูแลระบบคนสุดท้ายได้")
		}
	}
	return nil
}

func (uc *UseCase) validateStatusChange(ctx context.Context, user *userdomain.User, actorID uuid.UUID, newStatus string) error {
	if err := validateStatus(newStatus); err != nil {
		return err
	}
	if user.ID == actorID && newStatus != userdomain.StatusActive {
		return apperrors.ValidationError("ไม่สามารถเปลี่ยนสถานะบัญชีของตนเองได้")
	}
	if user.Role == userdomain.RoleAdmin && newStatus != userdomain.StatusActive {
		count, err := uc.users.CountActiveAdminsExcept(ctx, user.ID)
		if err != nil {
			return err
		}
		if count < 1 {
			return apperrors.ValidationError("ไม่สามารถระงับหรือปิดใช้งานผู้ดูแลระบบคนสุดท้ายได้")
		}
	}
	return nil
}

func validateRole(role string) error {
	switch role {
	case userdomain.RoleUser, userdomain.RoleAdmin:
		return nil
	default:
		return apperrors.ValidationError("บทบาทไม่ถูกต้อง")
	}
}

func validateStatus(status string) error {
	switch status {
	case userdomain.StatusActive, userdomain.StatusSuspended, userdomain.StatusDisabled:
		return nil
	default:
		return apperrors.ValidationError("สถานะไม่ถูกต้อง")
	}
}

func userSnapshot(u *userdomain.User) map[string]any {
	if u == nil {
		return nil
	}
	snap := map[string]any{
		"id":           u.ID.String(),
		"email":        u.Email,
		"display_name": u.DisplayName,
		"role":         u.Role,
		"status":       u.Status,
	}
	if u.LastLoginAt != nil {
		snap["last_login_at"] = u.LastLoginAt.UTC().Format(time.RFC3339)
	}
	return snap
}

func defaultAccessSummary(ctx context.Context, uc *UseCase, userID uuid.UUID) entdomain.AccessSummary {
	summaries, err := uc.entitlements.SummarizeActiveByUserIDs(ctx, []uuid.UUID{userID}, time.Now().UTC())
	if err != nil {
		return entdomain.BuildAccessSummary(false, 0, nil)
	}
	summary := summaries[userID]
	return entdomain.BuildAccessSummary(summary.HasPremium, summary.ActiveExamSetCount, summary.PremiumExpiresAt)
}

func toAccessSummaryResponse(summary entdomain.AccessSummary) AccessSummaryResponse {
	var premiumExpires *string
	if summary.PremiumExpiresAt != nil {
		s := summary.PremiumExpiresAt.UTC().Format(time.RFC3339)
		premiumExpires = &s
	}
	return AccessSummaryResponse{
		DisplayAccessType:  summary.DisplayAccessType,
		HasPremium:         summary.HasPremium,
		ActiveExamSetCount: summary.ActiveExamSetCount,
		PremiumExpiresAt:   premiumExpires,
	}
}

func toUserResponse(u userdomain.User, summary entdomain.AccessSummary) UserResponse {
	var lastLogin *string
	if u.LastLoginAt != nil {
		s := u.LastLoginAt.UTC().Format(time.RFC3339)
		lastLogin = &s
	}
	return UserResponse{
		ID:            u.ID.String(),
		Email:         u.Email,
		DisplayName:   u.DisplayName,
		Role:          u.Role,
		Status:        u.Status,
		LastLoginAt:   lastLogin,
		CreatedAt:     u.CreatedAt.UTC().Format(time.RFC3339),
		AccessSummary: toAccessSummaryResponse(summary),
	}
}

func toAccessSummaries(logs []accessdomain.AccessLog) []AccessLogSummary {
	resp := make([]AccessLogSummary, len(logs))
	for i, log := range logs {
		resp[i] = AccessLogSummary{
			ID:        log.ID.String(),
			EventType: log.EventType,
			Success:   log.Success,
			IPAddress: log.IPAddress,
			Message:   log.Message,
			CreatedAt: log.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	return resp
}

func toAuditSummaries(logs []auditdomain.AuditLog) []AuditLogSummary {
	resp := make([]AuditLogSummary, len(logs))
	for i, log := range logs {
		resp[i] = AuditLogSummary{
			ID:           log.ID.String(),
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			CreatedAt:    log.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	return resp
}
