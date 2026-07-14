package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

func TestReconcileSeasonUsesBangkokWindowAndReturnsCounts(t *testing.T) {
	t.Parallel()

	fixture := newReconcileRepositoryFixture()
	uc := NewLeaderboardUseCase(fixture)
	at := time.Date(2026, time.June, 30, 17, 30, 0, 0, time.UTC)

	got, err := uc.ReconcileSeason(t.Context(), fixture.trackID, at)
	if err != nil {
		t.Fatalf("ReconcileSeason() error = %v", err)
	}
	if fixture.calls != 1 {
		t.Fatalf("repository calls = %d, want 1", fixture.calls)
	}
	if fixture.window.Year != 2026 || fixture.window.Month != 7 {
		t.Errorf("repository window = %d-%02d, want 2026-07", fixture.window.Year, fixture.window.Month)
	}
	if got.SeasonID != fixture.result.SeasonID ||
		got.ScoreCount != fixture.result.ScoreCount ||
		got.EntryCount != fixture.result.EntryCount {
		t.Errorf("ReconcileSeason() = %+v, want %+v", got, fixture.result)
	}
}

type reconcileRepositoryFixture struct {
	leaderboardrepo.Repository

	trackID uuid.UUID
	window  domain.SeasonWindow
	result  leaderboardrepo.ReconcileResult
	calls   int
}

func newReconcileRepositoryFixture() *reconcileRepositoryFixture {
	return &reconcileRepositoryFixture{
		trackID: uuid.MustParse("51000000-0000-0000-0000-000000000001"),
		result: leaderboardrepo.ReconcileResult{
			SeasonID:   uuid.MustParse("52000000-0000-0000-0000-000000000002"),
			ScoreCount: 7,
			EntryCount: 4,
		},
	}
}

func (f *reconcileRepositoryFixture) ReconcileSeason(
	_ context.Context,
	trackID uuid.UUID,
	window domain.SeasonWindow,
) (*leaderboardrepo.ReconcileResult, error) {
	if trackID != f.trackID {
		return nil, context.Canceled
	}
	f.calls++
	f.window = window
	result := f.result
	return &result, nil
}
