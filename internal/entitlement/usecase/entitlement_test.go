package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	entdomain "virtual-exam-api/internal/entitlement/domain"
	entrepo "virtual-exam-api/internal/entitlement/repository"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

type fakeEntitlementRepo struct {
	examSetEntitlements map[uuid.UUID]map[uuid.UUID]*entdomain.Entitlement
	premiumEntitlements map[uuid.UUID]*entdomain.Entitlement
}

func newFakeEntitlementRepo() *fakeEntitlementRepo {
	return &fakeEntitlementRepo{
		examSetEntitlements: make(map[uuid.UUID]map[uuid.UUID]*entdomain.Entitlement),
		premiumEntitlements: make(map[uuid.UUID]*entdomain.Entitlement),
	}
}

func (f *fakeEntitlementRepo) Create(context.Context, *entdomain.Entitlement) error { return nil }
func (f *fakeEntitlementRepo) FindByID(context.Context, uuid.UUID) (*entdomain.Entitlement, error) {
	return nil, nil
}
func (f *fakeEntitlementRepo) Revoke(context.Context, uuid.UUID) error { return nil }
func (f *fakeEntitlementRepo) ListByUserID(context.Context, uuid.UUID, int, int) ([]entdomain.Entitlement, int64, error) {
	return nil, 0, nil
}
func (f *fakeEntitlementRepo) SummarizeActiveByUserIDs(context.Context, []uuid.UUID, time.Time) (map[uuid.UUID]entrepo.UserEntitlementSummary, error) {
	return nil, nil
}
func (f *fakeEntitlementRepo) FindActiveExamSetEntitlementForUpdate(context.Context, uuid.UUID, uuid.UUID) (*entdomain.Entitlement, error) {
	return nil, nil
}
func (f *fakeEntitlementRepo) ListAccessibleExamSets(context.Context, uuid.UUID, time.Time) ([]entdomain.AccessibleExamSetRow, error) {
	return nil, nil
}

func (f *fakeEntitlementRepo) FindActiveExamSetEntitlement(_ context.Context, userID, examSetID uuid.UUID, now time.Time) (*entdomain.Entitlement, error) {
	if bySet, ok := f.examSetEntitlements[userID]; ok {
		if ent, ok := bySet[examSetID]; ok && ent.IsCurrentlyActive(now) {
			return ent, nil
		}
	}
	return nil, nil
}

func (f *fakeEntitlementRepo) FindActivePremiumEntitlement(_ context.Context, userID uuid.UUID, now time.Time) (*entdomain.Entitlement, error) {
	if ent, ok := f.premiumEntitlements[userID]; ok && ent.IsCurrentlyActive(now) {
		return ent, nil
	}
	return nil, nil
}

func (f *fakeEntitlementRepo) HasActiveExamSetEntitlement(ctx context.Context, userID, examSetID uuid.UUID) (bool, error) {
	ent, err := f.FindActiveExamSetEntitlement(ctx, userID, examSetID, time.Now().UTC())
	return ent != nil, err
}

func (f *fakeEntitlementRepo) HasActivePremiumEntitlement(ctx context.Context, userID uuid.UUID) (bool, *time.Time, error) {
	ent, err := f.FindActivePremiumEntitlement(ctx, userID, time.Now().UTC())
	if err != nil || ent == nil {
		return false, nil, err
	}
	return true, ent.ExpiresAt, nil
}

func (f *fakeEntitlementRepo) grantExamSet(userID, examSetID uuid.UUID, expiresAt *time.Time) {
	if f.examSetEntitlements[userID] == nil {
		f.examSetEntitlements[userID] = make(map[uuid.UUID]*entdomain.Entitlement)
	}
	now := time.Now().UTC()
	refType := entdomain.RefTypeExamSet
	f.examSetEntitlements[userID][examSetID] = &entdomain.Entitlement{
		ID:              uuid.New(),
		UserID:          userID,
		EntitlementType: entdomain.TypeExamSet,
		RefType:         &refType,
		RefID:           &examSetID,
		StartsAt:        now.Add(-time.Hour),
		ExpiresAt:       expiresAt,
		IsActive:        true,
	}
}

func (f *fakeEntitlementRepo) grantPremium(userID uuid.UUID, expiresAt time.Time) {
	now := time.Now().UTC()
	f.premiumEntitlements[userID] = &entdomain.Entitlement{
		ID:              uuid.New(),
		UserID:          userID,
		EntitlementType: entdomain.TypePremium,
		StartsAt:        now.Add(-time.Hour),
		ExpiresAt:       &expiresAt,
		IsActive:        true,
	}
}

func publishedExamSet() *examsetdomain.ExamSet {
	return &examsetdomain.ExamSet{
		ID:             uuid.New(),
		Code:           "test-set",
		Title:          "Test Set",
		PriceAmount:    99,
		Currency:       "THB",
		Status:         examsetdomain.StatusPublished,
		IsActive:       true,
		TotalQuestions: 10,
	}
}

func reasonOf(result entdomain.ExamSetAccessResult) string {
	if result.Reason == nil {
		return ""
	}
	return *result.Reason
}

func TestCheckExamSetAccess(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	userPtr := &userID

	tests := []struct {
		name       string
		mutate     func(*examsetdomain.ExamSet)
		setup      func(*fakeEntitlementRepo, *examsetdomain.ExamSet)
		userID     *uuid.UUID
		questions  int
		wantStart  bool
		wantReason string
	}{
		{
			name: "free allowed",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessFree
			},
			userID:    userPtr,
			questions: 5,
			wantStart: true,
		},
		{
			name: "paid without entitlement denied",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPaid
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonAccessRequired,
		},
		{
			name: "paid with active exam_set entitlement allowed",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPaid
			},
			setup: func(f *fakeEntitlementRepo, set *examsetdomain.ExamSet) {
				f.grantExamSet(userID, set.ID, nil)
			},
			userID:    userPtr,
			questions: 5,
			wantStart: true,
		},
		{
			name: "paid with expired entitlement denied",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPaid
			},
			setup: func(f *fakeEntitlementRepo, set *examsetdomain.ExamSet) {
				expired := time.Now().UTC().Add(-time.Hour)
				f.grantExamSet(userID, set.ID, &expired)
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonAccessRequired,
		},
		{
			name: "premium without premium denied",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPremium
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonPremiumRequired,
		},
		{
			name: "premium with active premium allowed",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPremium
			},
			setup: func(f *fakeEntitlementRepo, _ *examsetdomain.ExamSet) {
				f.grantPremium(userID, time.Now().UTC().Add(24*time.Hour))
			},
			userID:    userPtr,
			questions: 5,
			wantStart: true,
		},
		{
			name: "premium single purchase with exam_set entitlement allowed",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPremium
				s.AllowSinglePurchase = true
			},
			setup: func(f *fakeEntitlementRepo, set *examsetdomain.ExamSet) {
				f.grantExamSet(userID, set.ID, nil)
			},
			userID:    userPtr,
			questions: 5,
			wantStart: true,
		},
		{
			name: "premium single purchase without access denied dual",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPremium
				s.AllowSinglePurchase = true
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonAccessRequiredOrPremium,
		},
		{
			name: "private without exam_set entitlement denied",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPrivate
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonPrivateExamAccessRequired,
		},
		{
			name: "private with exam_set entitlement allowed",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPrivate
			},
			setup: func(f *fakeEntitlementRepo, set *examsetdomain.ExamSet) {
				f.grantExamSet(userID, set.ID, nil)
			},
			userID:    userPtr,
			questions: 5,
			wantStart: true,
		},
		{
			name: "private with premium only denied",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessPrivate
			},
			setup: func(f *fakeEntitlementRepo, _ *examsetdomain.ExamSet) {
				f.grantPremium(userID, time.Now().UTC().Add(24*time.Hour))
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonPrivateExamAccessRequired,
		},
		{
			name: "inactive exam not available",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessFree
				s.IsActive = false
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonExamNotAvailable,
		},
		{
			name: "draft exam not available",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessFree
				s.Status = examsetdomain.StatusDraft
			},
			userID:     userPtr,
			questions:  5,
			wantReason: entdomain.ReasonExamNotAvailable,
		},
		{
			name: "no assigned questions not available",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessFree
			},
			userID:     userPtr,
			questions:  0,
			wantReason: entdomain.ReasonExamNotAvailable,
		},
		{
			name: "unauthenticated requires login",
			mutate: func(s *examsetdomain.ExamSet) {
				s.AccessType = examsetdomain.AccessFree
			},
			userID:     nil,
			questions:  5,
			wantReason: entdomain.ReasonLoginRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeEntitlementRepo()
			set := publishedExamSet()
			if tt.mutate != nil {
				tt.mutate(set)
			}
			if tt.setup != nil {
				tt.setup(repo, set)
			}

			uc := NewUseCase(repo, nil, nil)
			got := uc.CheckExamSetAccessWithQuestionCount(ctx, tt.userID, set, tt.questions)

			if got.CanStart != tt.wantStart {
				t.Fatalf("CanStart = %v, want %v", got.CanStart, tt.wantStart)
			}
			if reasonOf(got) != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", reasonOf(got), tt.wantReason)
			}
		})
	}
}

func TestAccessDeniedError(t *testing.T) {
	uc := NewUseCase(newFakeEntitlementRepo(), nil, nil)
	set := publishedExamSet()
	set.AccessType = examsetdomain.AccessPaid

	check := entdomain.ExamSetAccessResult{
		Reason: strPtr(entdomain.ReasonAccessRequired),
	}
	err := uc.AccessDeniedError(set, check)
	if err.Code != "ACCESS_REQUIRED" {
		t.Fatalf("code = %q", err.Code)
	}

	premiumCheck := entdomain.ExamSetAccessResult{
		Reason: strPtr(entdomain.ReasonPremiumRequired),
	}
	set.AccessType = examsetdomain.AccessPremium
	premiumErr := uc.AccessDeniedError(set, premiumCheck)
	if premiumErr.Code != "PREMIUM_REQUIRED" {
		t.Fatalf("code = %q", premiumErr.Code)
	}
	details, ok := premiumErr.Details.(map[string]any)
	if !ok || details["pricing_url"] != "/pricing" {
		t.Fatalf("expected pricing_url in details")
	}
}
