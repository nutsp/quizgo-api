package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

func TestFinalizeDueSeasonsFinalizesEachDueSeasonAndIsRetryIdempotent(t *testing.T) {
	fixture := newFinalizerRepositoryFixture()
	uc := NewLeaderboardUseCase(fixture)

	count, err := uc.FinalizeDueSeasons(t.Context(), fixture.now)
	if err != nil {
		t.Fatalf("FinalizeDueSeasons() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("FinalizeDueSeasons() count = %d, want 2", count)
	}
	if len(fixture.finalizedIDs) != 2 || fixture.finalizedIDs[0] != fixture.due[0].ID || fixture.finalizedIDs[1] != fixture.due[1].ID {
		t.Fatalf("finalized IDs = %v, want due order", fixture.finalizedIDs)
	}

	count, err = uc.FinalizeDueSeasons(t.Context(), fixture.now.Add(time.Minute))
	if err != nil {
		t.Fatalf("FinalizeDueSeasons() retry error = %v", err)
	}
	if count != 0 {
		t.Fatalf("FinalizeDueSeasons() retry count = %d, want 0", count)
	}
}

type finalizerRepositoryFixture struct {
	leaderboardrepo.Repository

	now          time.Time
	due          []leaderboardrepo.SeasonRow
	finalized    map[uuid.UUID]bool
	finalizedIDs []uuid.UUID
}

func newFinalizerRepositoryFixture() *finalizerRepositoryFixture {
	now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	return &finalizerRepositoryFixture{
		now: now,
		due: []leaderboardrepo.SeasonRow{
			{ID: uuid.New(), Year: 2026, Month: 5, Status: "active", EndsAt: now.Add(-2 * time.Hour)},
			{ID: uuid.New(), Year: 2026, Month: 6, Status: "active", EndsAt: now.Add(-time.Hour)},
		},
		finalized: make(map[uuid.UUID]bool),
	}
}

func (f *finalizerRepositoryFixture) ListDueSeasons(context.Context, time.Time) ([]leaderboardrepo.SeasonRow, error) {
	rows := make([]leaderboardrepo.SeasonRow, 0, len(f.due))
	for _, season := range f.due {
		if !f.finalized[season.ID] {
			rows = append(rows, season)
		}
	}
	return rows, nil
}

func (f *finalizerRepositoryFixture) FinalizeSeason(_ context.Context, seasonID uuid.UUID, finalizedAt time.Time) (*leaderboardrepo.FinalizationResult, error) {
	if f.finalized[seasonID] {
		return &leaderboardrepo.FinalizationResult{SeasonID: seasonID}, nil
	}
	f.finalized[seasonID] = true
	f.finalizedIDs = append(f.finalizedIDs, seasonID)
	return &leaderboardrepo.FinalizationResult{
		SeasonID: seasonID, Finalized: true, FinalizedAt: finalizedAt, AwardCount: 3,
	}, nil
}
