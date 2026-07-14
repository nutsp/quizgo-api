package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
)

type ReconcileSummary struct {
	SeasonID   uuid.UUID
	ScoreCount int64
	EntryCount int64
}

func (uc *LeaderboardUseCase) ReconcileSeason(ctx context.Context, trackID uuid.UUID, at time.Time) (ReconcileSummary, error) {
	window, err := domain.BangkokSeasonWindow(at)
	if err != nil {
		return ReconcileSummary{}, err
	}
	result, err := uc.repo.ReconcileSeason(ctx, trackID, window)
	if err != nil {
		return ReconcileSummary{}, err
	}
	return ReconcileSummary{
		SeasonID:   result.SeasonID,
		ScoreCount: result.ScoreCount,
		EntryCount: result.EntryCount,
	}, nil
}
