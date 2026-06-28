package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/entitlement/domain"
)

func (uc *UseCase) ListMyExams(ctx context.Context, userID uuid.UUID) (*domain.MyExamsResponse, error) {
	now := time.Now().UTC()

	premiumEnt, _ := uc.entitlements.FindActivePremiumEntitlement(ctx, userID, now)
	hasPremium := premiumEnt != nil
	var premiumExpiresAt *string
	if premiumEnt != nil && premiumEnt.ExpiresAt != nil {
		s := premiumEnt.ExpiresAt.UTC().Format(time.RFC3339)
		premiumExpiresAt = &s
	}

	rows, err := uc.entitlements.ListAccessibleExamSets(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	items := make([]domain.MyExamItem, 0, len(rows))
	privateCount := 0
	for _, row := range rows {
		if row.ExamSet.AccessType == "private" {
			privateCount++
		}
		items = append(items, toMyExamItem(row, now))
	}

	return &domain.MyExamsResponse{
		Summary: domain.MyExamSummary{
			HasPremium:           hasPremium,
			PremiumExpiresAt:     premiumExpiresAt,
			UnlockedExamSetCount: len(items),
			PrivateExamSetCount:  privateCount,
		},
		Items: items,
	}, nil
}

func toMyExamItem(row domain.AccessibleExamSetRow, now time.Time) domain.MyExamItem {
	set := row.ExamSet
	ent := row.Entitlement

	var expiresAt *string
	if ent.ExpiresAt != nil {
		s := ent.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}

	item := domain.MyExamItem{
		ID:              set.ID.String(),
		Code:            set.Code,
		Title:           set.Title,
		Description:     set.Description,
		AccessType:      set.AccessType,
		AccessSource:    domain.ResolveAccessSource(set.AccessType),
		CoverImageURL:   set.CoverImageURL,
		TotalQuestions:  set.TotalQuestions,
		DurationMinutes: set.DurationMinutes,
		Difficulty:      set.Difficulty,
		PassingScore:    set.PassingScore,
		Entitlement: domain.MyExamEntitlement{
			ID:        ent.ID.String(),
			Source:    ent.Source,
			StartsAt:  ent.StartsAt.UTC().Format(time.RFC3339),
			ExpiresAt: expiresAt,
			Status:    ent.Status(now),
		},
	}
	if set.ExamTrack != nil {
		item.ExamTrack = &domain.MyExamTrackRef{
			ID:   set.ExamTrackID.String(),
			Name: set.ExamTrack.Name,
			Code: set.ExamTrack.Code,
		}
	}
	if row.AttemptID != nil && row.AttemptStatus != nil {
		attempt := domain.MyExamLatestAttempt{
			AttemptID: row.AttemptID.String(),
			Status:    *row.AttemptStatus,
		}
		if row.ScorePercent != nil {
			attempt.ScorePercent = row.ScorePercent
		}
		if row.AttemptSubmittedAt != nil {
			s := row.AttemptSubmittedAt.UTC().Format(time.RFC3339)
			attempt.SubmittedAt = &s
		}
		item.LatestAttempt = &attempt
	}
	return item
}
