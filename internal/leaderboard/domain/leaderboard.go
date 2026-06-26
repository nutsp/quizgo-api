package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Pagination struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

type ExamSetRef struct {
	Code          string `json:"code"`
	Title         string `json:"title"`
	ExamTrackName string `json:"exam_track_name"`
}

type ExamTrackRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ExamSetLeaderboardEntry struct {
	Rank            int        `json:"rank"`
	UserID          string     `json:"user_id"`
	DisplayName     string     `json:"display_name"`
	IsCurrentUser   bool       `json:"is_current_user"`
	Score           float64    `json:"score"`
	TotalScore      float64    `json:"total_score"`
	ScorePercent    float64    `json:"score_percent"`
	Passed          bool       `json:"passed"`
	DurationSeconds int        `json:"duration_seconds"`
	SubmittedAt     *time.Time `json:"submitted_at"`
}

type ExamSetCurrentUserRank struct {
	Rank            int        `json:"rank"`
	ScorePercent    float64    `json:"score_percent"`
	DurationSeconds int        `json:"duration_seconds"`
	SubmittedAt     *time.Time `json:"submitted_at"`
}

type ExamSetLeaderboardResponse struct {
	ExamSet         ExamSetRef              `json:"exam_set"`
	Leaderboard     []ExamSetLeaderboardEntry `json:"leaderboard"`
	CurrentUserRank *ExamSetCurrentUserRank `json:"current_user_rank,omitempty"`
	Pagination      Pagination              `json:"pagination"`
}

type ExamTrackLeaderboardEntry struct {
	Rank                int        `json:"rank"`
	UserID              string     `json:"user_id"`
	DisplayName         string     `json:"display_name"`
	IsCurrentUser       bool       `json:"is_current_user"`
	AverageScorePercent float64    `json:"average_score_percent"`
	CompletedExamSets   int        `json:"completed_exam_sets"`
	PassedExamSets      int        `json:"passed_exam_sets"`
	PassRatePercent     float64    `json:"pass_rate_percent"`
	LatestSubmittedAt   *time.Time `json:"latest_submitted_at"`
}

type ExamTrackCurrentUserRank struct {
	Rank                int     `json:"rank"`
	AverageScorePercent float64 `json:"average_score_percent"`
	CompletedExamSets   int     `json:"completed_exam_sets"`
	PassedExamSets      int     `json:"passed_exam_sets"`
	PassRatePercent     float64 `json:"pass_rate_percent"`
}

type ExamTrackLeaderboardResponse struct {
	ExamTrack       ExamTrackRef               `json:"exam_track"`
	Leaderboard     []ExamTrackLeaderboardEntry `json:"leaderboard"`
	CurrentUserRank *ExamTrackCurrentUserRank  `json:"current_user_rank,omitempty"`
	Pagination      Pagination                 `json:"pagination"`
}

type ListFilter struct {
	Page  int
	Limit int
}

type ExamSetContext struct {
	ID              uuid.UUID
	Code            string
	Title           string
	ExamTrackName   string
	PassingScore    int
}

type ExamTrackContext struct {
	ID   uuid.UUID
	Code string
	Name string
}

func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		if len(local) == 0 {
			return "***@" + domain
		}
		return local[:1] + "***@" + domain
	}
	return local[:2] + "***@" + domain
}

func PublicDisplayName(displayName, email string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	return MaskEmail(email)
}
