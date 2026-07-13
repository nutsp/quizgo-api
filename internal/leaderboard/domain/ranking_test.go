package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAttemptWinsUsesPointsThenDurationThenAchievedAt(t *testing.T) {
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	current := ScoreCandidate{
		Points:          82,
		DurationSeconds: 900,
		AchievedAt:      base,
	}

	testCases := []struct {
		name      string
		candidate ScoreCandidate
		want      bool
	}{
		{"higher points", ScoreCandidate{Points: 83, DurationSeconds: 1200, AchievedAt: base.Add(time.Hour)}, true},
		{"lower points", ScoreCandidate{Points: 81, DurationSeconds: 600, AchievedAt: base.Add(-time.Hour)}, false},
		{"shorter duration", ScoreCandidate{Points: 82, DurationSeconds: 800, AchievedAt: base.Add(time.Hour)}, true},
		{"longer duration", ScoreCandidate{Points: 82, DurationSeconds: 1000, AchievedAt: base.Add(-time.Hour)}, false},
		{"earlier achievement", ScoreCandidate{Points: 82, DurationSeconds: 900, AchievedAt: base.Add(-time.Second)}, true},
		{"same candidate", current, false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := AttemptWins(testCase.candidate, current); got != testCase.want {
				t.Errorf("AttemptWins() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestLeaderboardEntryPreservesSharedRanks(t *testing.T) {
	entries := []LeaderboardEntry{
		{Rank: 1, TotalPoints: 100},
		{Rank: 2, TotalPoints: 90},
		{Rank: 2, TotalPoints: 90},
		{Rank: 4, TotalPoints: 80},
	}

	for index, want := range []int{1, 2, 2, 4} {
		if entries[index].Rank != want {
			t.Errorf("entries[%d].Rank = %d, want %d", index, entries[index].Rank, want)
		}
	}
}

func TestHubResponseJSONContract(t *testing.T) {
	encoded, err := json.Marshal(HubResponse{})
	if err != nil {
		t.Fatalf("json.Marshal(HubResponse{}) error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{
		"season",
		"exam_track",
		"current_user",
		"top_three",
		"leaderboard",
		"next_opportunities",
		"pagination",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("HubResponse JSON is missing %q", key)
		}
	}
}

func TestProjectionUpdateJSONContract(t *testing.T) {
	encoded, err := json.Marshal(ProjectionUpdate{})
	if err != nil {
		t.Fatalf("json.Marshal(ProjectionUpdate{}) error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{
		"season_id",
		"track_code",
		"year",
		"month",
		"points_added",
		"best_score_before",
		"best_score_after",
		"current_rank",
		"total_points",
		"improved_best_score",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("ProjectionUpdate JSON is missing %q", key)
		}
	}
	if _, ok := got["previous_rank"]; ok {
		t.Error("ProjectionUpdate JSON includes an empty previous_rank")
	}
}
