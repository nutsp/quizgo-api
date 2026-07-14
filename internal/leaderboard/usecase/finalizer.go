package usecase

import (
	"context"
	"time"
)

func (uc *LeaderboardUseCase) FinalizeDueSeasons(ctx context.Context, at time.Time) (int, error) {
	due, err := uc.repo.ListDueSeasons(ctx, at)
	if err != nil {
		return 0, err
	}

	finalized := 0
	for _, season := range due {
		result, err := uc.repo.FinalizeSeason(ctx, season.ID, at)
		if err != nil {
			return finalized, err
		}
		if !result.Finalized {
			continue
		}
		finalized++
		uc.logger.Printf(
			"leaderboard season finalized season_id=%s year=%d month=%d awards=%d",
			season.ID,
			season.Year,
			season.Month,
			result.AwardCount,
		)
	}
	return finalized, nil
}
