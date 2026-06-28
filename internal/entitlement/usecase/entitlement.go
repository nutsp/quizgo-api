package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/entitlement/domain"
	entrepo "virtual-exam-api/internal/entitlement/repository"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	userrepo "virtual-exam-api/internal/user/repository"
)

type UseCase struct {
	entitlements entrepo.Repository
	examSets     examsetrepo.Repository
	users        userrepo.Repository
}

func NewUseCase(
	entitlements entrepo.Repository,
	examSets examsetrepo.Repository,
	users userrepo.Repository,
) *UseCase {
	return &UseCase{
		entitlements: entitlements,
		examSets:     examSets,
		users:        users,
	}
}

func (uc *UseCase) CheckExamSetAccess(ctx context.Context, userID *uuid.UUID, set *examsetdomain.ExamSet) domain.ExamSetAccessResult {
	return uc.CheckExamSetAccessWithQuestionCount(ctx, userID, set, -1)
}

func (uc *UseCase) CheckExamSetAccessWithQuestionCount(
	ctx context.Context,
	userID *uuid.UUID,
	set *examsetdomain.ExamSet,
	assignedQuestionCount int,
) domain.ExamSetAccessResult {
	result := domain.ExamSetAccessResult{
		AccessType:          set.AccessType,
		AllowSinglePurchase: set.AllowSinglePurchase,
		PriceAmount:         int(set.PriceAmount),
		PriceCurrency:       set.Currency,
		UnlockURL:           fmt.Sprintf("/exams/%s/unlock", set.Code),
	}

	if !isExamSetAvailable(set, assignedQuestionCount) {
		result.Reason = strPtr(domain.ReasonExamNotAvailable)
		return result
	}

	if userID == nil {
		result.Reason = strPtr(domain.ReasonLoginRequired)
		return result
	}

	now := time.Now().UTC()
	hasPremium := uc.hasActivePremium(ctx, *userID, now)
	hasExamSet := uc.hasActiveExamSet(ctx, *userID, set.ID, now)
	result.HasPremium = hasPremium
	result.HasExamSetAccess = hasExamSet

	switch set.AccessType {
	case examsetdomain.AccessFree:
		result.CanStart = true
		return result
	case examsetdomain.AccessPaid:
		if hasExamSet {
			result.CanStart = true
			return result
		}
		result.Reason = strPtr(domain.ReasonAccessRequired)
		return result
	case examsetdomain.AccessPremium:
		if hasPremium {
			result.CanStart = true
			result.AvailableOptions = []string{"premium"}
			return result
		}
		if set.AllowSinglePurchase && hasExamSet {
			result.CanStart = true
			result.AvailableOptions = []string{"single_purchase"}
			return result
		}
		if set.AllowSinglePurchase {
			result.Reason = strPtr(domain.ReasonAccessRequiredOrPremium)
			result.AvailableOptions = []string{"single_purchase", "premium"}
			return result
		}
		result.Reason = strPtr(domain.ReasonPremiumRequired)
		result.UnlockURL = "/pricing"
		return result
	case examsetdomain.AccessPrivate:
		if hasExamSet {
			result.CanStart = true
			return result
		}
		result.Reason = strPtr(domain.ReasonPrivateExamAccessRequired)
		return result
	default:
		result.Reason = strPtr(domain.ReasonAccessRequired)
		return result
	}
}

func isExamSetAvailable(set *examsetdomain.ExamSet, assignedQuestionCount int) bool {
	if set.Status != examsetdomain.StatusPublished || !set.IsActive {
		return false
	}
	if assignedQuestionCount >= 0 {
		return assignedQuestionCount > 0
	}
	return set.TotalQuestions > 0
}

func strPtr(s string) *string {
	return &s
}

func (uc *UseCase) HasActiveExamSetEntitlement(ctx context.Context, userID, examSetID uuid.UUID) (bool, error) {
	return uc.entitlements.HasActiveExamSetEntitlement(ctx, userID, examSetID)
}

func (uc *UseCase) HasActivePremiumEntitlement(ctx context.Context, userID uuid.UUID) (bool, *time.Time, error) {
	return uc.entitlements.HasActivePremiumEntitlement(ctx, userID)
}

func (uc *UseCase) BuildAccessInfo(ctx context.Context, userID *uuid.UUID, set *examsetdomain.ExamSet) examsetdomain.AccessInfo {
	return uc.BuildAccessInfoWithQuestionCount(ctx, userID, set, -1)
}

func (uc *UseCase) BuildAccessInfoWithQuestionCount(
	ctx context.Context,
	userID *uuid.UUID,
	set *examsetdomain.ExamSet,
	assignedQuestionCount int,
) examsetdomain.AccessInfo {
	check := uc.CheckExamSetAccessWithQuestionCount(ctx, userID, set, assignedQuestionCount)
	return examsetdomain.AccessInfo{
		CanStart:         check.CanStart,
		Reason:           check.Reason,
		HasExamSetAccess: check.HasExamSetAccess,
		HasPremium:       check.HasPremium,
		AvailableOptions: check.AvailableOptions,
	}
}

func (uc *UseCase) AccessDeniedError(set *examsetdomain.ExamSet, check domain.ExamSetAccessResult) *apperrors.AppError {
	if check.Reason == nil {
		return apperrors.ErrLoginRequired
	}

	switch *check.Reason {
	case domain.ReasonExamNotAvailable:
		return apperrors.ErrExamNotAvailable
	case domain.ReasonPremiumRequired:
		return apperrors.NewWithDetails(
			"PREMIUM_REQUIRED",
			"ชุดข้อสอบนี้สำหรับสมาชิก Premium เท่านั้น",
			403,
			map[string]any{
				"access_type": set.AccessType,
				"pricing_url": "/pricing",
			},
		)
	case domain.ReasonAccessRequired:
		return apperrors.NewWithDetails(
			"ACCESS_REQUIRED",
			"ชุดข้อสอบนี้ต้องปลดล็อกก่อนเริ่มทำข้อสอบ",
			403,
			map[string]any{
				"access_type":    set.AccessType,
				"price_amount":   int(set.PriceAmount),
				"price_currency": set.Currency,
				"unlock_url":     check.UnlockURL,
			},
		)
	case domain.ReasonAccessRequiredOrPremium:
		return apperrors.NewWithDetails(
			"ACCESS_REQUIRED_OR_PREMIUM",
			"ชุดข้อสอบนี้สามารถปลดล็อกเฉพาะชุด หรือใช้งานผ่าน Premium ได้",
			403,
			map[string]any{
				"access_type":           set.AccessType,
				"allow_single_purchase": set.AllowSinglePurchase,
				"price_amount":          int(set.PriceAmount),
				"price_currency":        set.Currency,
				"unlock_url":            check.UnlockURL,
				"pricing_url":           "/pricing",
			},
		)
	case domain.ReasonPrivateExamAccessRequired:
		return apperrors.ErrPrivateExamAccessRequired
	case domain.ReasonLoginRequired:
		return apperrors.ErrLoginRequired
	default:
		return apperrors.ErrLoginRequired
	}
}

func (uc *UseCase) GrantExamSetAccess(ctx context.Context, input domain.GrantExamSetAccessInput) (*domain.Entitlement, error) {
	if input.ExamSetID == uuid.Nil || input.UserID == uuid.Nil {
		return nil, apperrors.ErrInvalidEntitlement
	}
	set, err := uc.examSets.FindByID(ctx, input.ExamSetID)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC()) {
		return nil, apperrors.ValidationError("วันหมดอายุต้องอยู่ในอนาคต")
	}

	existing, err := uc.entitlements.FindActiveExamSetEntitlementForUpdate(ctx, input.UserID, input.ExamSetID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.ErrEntitlementAlreadyExists
	}

	source := input.Source
	if source == "" {
		source = domain.SourceManual
	}

	now := time.Now().UTC()
	refType := domain.RefTypeExamSet
	ent := &domain.Entitlement{
		ID:              uuid.New(),
		UserID:          input.UserID,
		EntitlementType: domain.TypeExamSet,
		RefType:         &refType,
		RefID:           &input.ExamSetID,
		Source:          source,
		StartsAt:        now,
		ExpiresAt:       input.ExpiresAt,
		IsActive:        true,
		Notes:           input.Notes,
		GrantedBy:       &input.GrantedBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := uc.entitlements.Create(ctx, ent); err != nil {
		return nil, err
	}
	ent.RefName = &set.Title
	return ent, nil
}

func (uc *UseCase) GrantPremiumAccess(ctx context.Context, input domain.GrantPremiumAccessInput) (*domain.Entitlement, error) {
	if input.UserID == uuid.Nil {
		return nil, apperrors.ErrInvalidEntitlement
	}
	if !input.ExpiresAt.After(time.Now().UTC()) {
		return nil, apperrors.ValidationError("วันหมดอายุต้องอยู่ในอนาคต")
	}

	source := input.Source
	if source == "" {
		source = domain.SourceManual
	}

	now := time.Now().UTC()
	ent := &domain.Entitlement{
		ID:              uuid.New(),
		UserID:          input.UserID,
		EntitlementType: domain.TypePremium,
		Source:          source,
		StartsAt:        now,
		ExpiresAt:       &input.ExpiresAt,
		IsActive:        true,
		Notes:           input.Notes,
		GrantedBy:       &input.GrantedBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := uc.entitlements.Create(ctx, ent); err != nil {
		return nil, err
	}
	return ent, nil
}

func (uc *UseCase) RevokeEntitlement(ctx context.Context, entitlementID, actorID uuid.UUID) (*domain.Entitlement, error) {
	ent, err := uc.entitlements.FindByID(ctx, entitlementID)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, apperrors.ErrEntitlementNotFound
	}
	if !ent.IsActive {
		return nil, apperrors.ErrEntitlementNotFound
	}
	if err := uc.entitlements.Revoke(ctx, entitlementID); err != nil {
		return nil, err
	}
	ent.IsActive = false
	return ent, nil
}

func (uc *UseCase) ListUserEntitlements(ctx context.Context, userID uuid.UUID, page, limit int) (*domain.PaginatedEntitlements, error) {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.ErrNotFound
	}

	page, limit = pagination.Sanitize(page, limit)
	items, total, err := uc.entitlements.ListByUserID(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	for i := range items {
		uc.enrichEntitlement(ctx, &items[i], now)
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if total == 0 {
		totalPages = 0
	}
	return &domain.PaginatedEntitlements{
		Items:      items,
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

func (uc *UseCase) enrichEntitlement(ctx context.Context, ent *domain.Entitlement, now time.Time) {
	if ent.RefID != nil && ent.RefType != nil && *ent.RefType == domain.RefTypeExamSet {
		if set, _ := uc.examSets.FindByID(ctx, *ent.RefID); set != nil {
			ent.RefName = &set.Title
		}
	}
	if ent.GrantedBy != nil {
		if u, _ := uc.users.FindByID(ctx, *ent.GrantedBy); u != nil {
			name := u.DisplayName
			if name == "" {
				name = u.Email
			}
			ent.GrantedByName = &name
		}
	}
	_ = ent.Status(now)
}

func (uc *UseCase) hasActivePremium(ctx context.Context, userID uuid.UUID, now time.Time) bool {
	ent, err := uc.entitlements.FindActivePremiumEntitlement(ctx, userID, now)
	return err == nil && ent != nil
}

func (uc *UseCase) hasActiveExamSet(ctx context.Context, userID, examSetID uuid.UUID, now time.Time) bool {
	ent, err := uc.entitlements.FindActiveExamSetEntitlement(ctx, userID, examSetID, now)
	return err == nil && ent != nil
}

type EntitlementResponse struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	EntitlementType string  `json:"entitlement_type"`
	RefType         *string `json:"ref_type,omitempty"`
	RefID           *string `json:"ref_id,omitempty"`
	RefName         *string `json:"ref_name,omitempty"`
	Source          string  `json:"source"`
	StartsAt        string  `json:"starts_at"`
	ExpiresAt       *string `json:"expires_at,omitempty"`
	IsActive        bool    `json:"is_active"`
	Status          string  `json:"status"`
	Notes           *string `json:"notes,omitempty"`
	GrantedBy       *string `json:"granted_by,omitempty"`
	GrantedByName   *string `json:"granted_by_name,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

func ToEntitlementResponse(ent domain.Entitlement) EntitlementResponse {
	now := time.Now().UTC()
	var refID *string
	if ent.RefID != nil {
		s := ent.RefID.String()
		refID = &s
	}
	var grantedBy *string
	if ent.GrantedBy != nil {
		s := ent.GrantedBy.String()
		grantedBy = &s
	}
	var expiresAt *string
	if ent.ExpiresAt != nil {
		s := ent.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}
	return EntitlementResponse{
		ID:              ent.ID.String(),
		UserID:          ent.UserID.String(),
		EntitlementType: ent.EntitlementType,
		RefType:         ent.RefType,
		RefID:           refID,
		RefName:         ent.RefName,
		Source:          ent.Source,
		StartsAt:        ent.StartsAt.UTC().Format(time.RFC3339),
		ExpiresAt:       expiresAt,
		IsActive:        ent.IsActive,
		Status:          ent.Status(now),
		Notes:           ent.Notes,
		GrantedBy:       grantedBy,
		GrantedByName:   ent.GrantedByName,
		CreatedAt:       ent.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func ToEntitlementResponses(items []domain.Entitlement) []EntitlementResponse {
	out := make([]EntitlementResponse, len(items))
	for i, item := range items {
		out[i] = ToEntitlementResponse(item)
	}
	return out
}
