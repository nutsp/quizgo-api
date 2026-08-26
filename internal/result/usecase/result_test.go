package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/result/domain"
	resultrepo "virtual-exam-api/internal/result/repository"
)

type guardedResultRepo struct{ resultrepo.Repository }

type premiumChecker struct{ active bool }

func (c premiumChecker) HasActivePremiumEntitlement(context.Context, uuid.UUID) (bool, *time.Time, error) {
	return c.active, nil, nil
}

func TestFreeUsersCannotReadResultHistoryOrProgress(t *testing.T) {
	uc := NewResultUseCase(guardedResultRepo{}, premiumChecker{active: false})
	userID := uuid.New()
	tests := []struct {
		name string
		run  func() error
	}{
		{"overall summary", func() error { _, err := uc.GetMyResultsSummary(context.Background(), userID); return err }},
		{"track list", func() error { _, err := uc.GetMyExamTrackResults(context.Background(), userID); return err }},
		{"track detail", func() error {
			_, err := uc.GetMyExamTrackResultDetail(context.Background(), userID, "track")
			return err
		}},
		{"attempt history", func() error {
			_, err := uc.ListMyAttemptResults(context.Background(), userID, domain.AttemptHistoryFilter{})
			return err
		}},
		{"set progress", func() error { _, err := uc.GetMyExamSetResultDetail(context.Background(), userID, "set"); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appErr *apperrors.AppError
			if err := tt.run(); !errors.As(err, &appErr) || appErr.Code != "PREMIUM_REQUIRED" {
				t.Fatalf("error = %#v, want PREMIUM_REQUIRED", err)
			}
		})
	}
}
