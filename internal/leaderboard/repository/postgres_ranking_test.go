package repository

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMonthlyRankingCalculatesSharedRanksAndStableUserOrder(t *testing.T) {
	tiedAt := time.Date(2026, time.July, 10, 10, 0, 0, 0, time.UTC)
	rows := []rankingFixtureRow{
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000004"), TotalPoints: 80, CompletedExamSets: 1, TotalDurationSeconds: 700, ScoreAchievedAt: tiedAt},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), TotalPoints: 90, CompletedExamSets: 1, TotalDurationSeconds: 600, ScoreAchievedAt: tiedAt},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), TotalPoints: 100, CompletedExamSets: 1, TotalDurationSeconds: 500, ScoreAchievedAt: tiedAt},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), TotalPoints: 90, CompletedExamSets: 1, TotalDurationSeconds: 600, ScoreAchievedAt: tiedAt},
	}

	got := calculateFixtureRanks(rows)
	for index, wantRank := range []int{1, 2, 2, 4} {
		if got[index].Rank != wantRank {
			t.Errorf("row %d rank = %d, want %d", index, got[index].Rank, wantRank)
		}
	}
	if got[1].UserID.String() >= got[2].UserID.String() {
		t.Errorf("tied users are not ordered by user ID: %s, %s", got[1].UserID, got[2].UserID)
	}

	window := monthlyRankWindowSQL
	if strings.Contains(window, "user_id") {
		t.Errorf("rank window uses user_id as a tie-breaker: %s", window)
	}
	wantWindow := "total_points DESC, completed_exam_sets DESC, total_duration_seconds ASC, score_achieved_at ASC"
	if gotWindow := strings.Join(strings.Fields(window), " "); gotWindow != wantWindow {
		t.Errorf("rank window = %q, want %q", gotWindow, wantWindow)
	}
	if !strings.Contains(monthlyRankedEntriesCTE, "ORDER BY rank, user_id") {
		t.Error("monthly ranking query does not stabilize equal ranks by user_id outside the window")
	}
}

type rankingFixtureRow struct {
	Rank                 int
	UserID               uuid.UUID
	TotalPoints          float64
	CompletedExamSets    int
	TotalDurationSeconds int64
	ScoreAchievedAt      time.Time
}

func calculateFixtureRanks(rows []rankingFixtureRow) []rankingFixtureRow {
	sort.Slice(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if left.TotalPoints != right.TotalPoints {
			return left.TotalPoints > right.TotalPoints
		}
		if left.CompletedExamSets != right.CompletedExamSets {
			return left.CompletedExamSets > right.CompletedExamSets
		}
		if left.TotalDurationSeconds != right.TotalDurationSeconds {
			return left.TotalDurationSeconds < right.TotalDurationSeconds
		}
		if !left.ScoreAchievedAt.Equal(right.ScoreAchievedAt) {
			return left.ScoreAchievedAt.Before(right.ScoreAchievedAt)
		}
		return left.UserID.String() < right.UserID.String()
	})

	for index := range rows {
		if index == 0 || !sameRankingFields(rows[index], rows[index-1]) {
			rows[index].Rank = index + 1
		} else {
			rows[index].Rank = rows[index-1].Rank
		}
	}
	return rows
}

func sameRankingFields(left, right rankingFixtureRow) bool {
	return left.TotalPoints == right.TotalPoints &&
		left.CompletedExamSets == right.CompletedExamSets &&
		left.TotalDurationSeconds == right.TotalDurationSeconds &&
		left.ScoreAchievedAt.Equal(right.ScoreAchievedAt)
}
