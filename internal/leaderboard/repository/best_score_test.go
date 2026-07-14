package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
)

func TestBestScoreUpdateExactTieReplacementIsNotImprovement(t *testing.T) {
	achievedAt := time.Date(2026, time.July, 15, 4, 0, 0, 0, time.UTC)
	previous := domain.ScoreCandidate{Points: 80, DurationSeconds: 600, AchievedAt: achievedAt}
	candidateAttemptID := uuid.MustParse("64000000-0000-0000-0000-000000000001")
	currentAttemptID := uuid.MustParse("64000000-0000-0000-0000-000000000002")

	got, replace := bestScoreUpdate(candidateAttemptID, currentAttemptID, previous, previous)
	if !replace {
		t.Fatal("bestScoreUpdate() replace = false, want true for lower attempt ID")
	}
	if got.Improved {
		t.Error("BestScoreUpdate.Improved = true, want false for exact-tie replacement")
	}
	if got.Current != previous {
		t.Errorf("BestScoreUpdate.Current = %+v, want %+v", got.Current, previous)
	}
}
