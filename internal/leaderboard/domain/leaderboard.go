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
	ExamSet         ExamSetRef                `json:"exam_set"`
	Leaderboard     []ExamSetLeaderboardEntry `json:"leaderboard"`
	CurrentUserRank *ExamSetCurrentUserRank   `json:"current_user_rank,omitempty"`
	Pagination      Pagination                `json:"pagination"`
}

// Deprecated: retained for the legacy track-average API until Task 5 migrates it.
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

// Deprecated: retained for the legacy track-average API until Task 5 migrates it.
type ExamTrackCurrentUserRank struct {
	Rank                int     `json:"rank"`
	AverageScorePercent float64 `json:"average_score_percent"`
	CompletedExamSets   int     `json:"completed_exam_sets"`
	PassedExamSets      int     `json:"passed_exam_sets"`
	PassRatePercent     float64 `json:"pass_rate_percent"`
}

// Deprecated: retained for the legacy track-average API until Task 5 migrates it.
type ExamTrackLeaderboardResponse struct {
	ExamTrack       ExamTrackRef                `json:"exam_track"`
	Leaderboard     []ExamTrackLeaderboardEntry `json:"leaderboard"`
	CurrentUserRank *ExamTrackCurrentUserRank   `json:"current_user_rank,omitempty"`
	Pagination      Pagination                  `json:"pagination"`
}

type SeasonWindow struct {
	Year        int        `json:"year"`
	Month       int        `json:"month"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	Status      string     `json:"status"`
	FinalizedAt *time.Time `json:"finalized_at"`
}

type ScoreCandidate struct {
	Points          float64
	DurationSeconds int
	AchievedAt      time.Time
}

type ProjectionInput struct {
	AttemptID   uuid.UUID
	UserID      uuid.UUID
	ExamSetID   uuid.UUID
	ExamTrackID uuid.UUID
	TrackCode   string
	SubmittedAt time.Time
	Candidate   ScoreCandidate
}

type ProjectionUpdate struct {
	SeasonID          string  `json:"season_id"`
	TrackCode         string  `json:"track_code"`
	Year              int     `json:"year"`
	Month             int     `json:"month"`
	PointsAdded       float64 `json:"points_added"`
	BestScoreBefore   float64 `json:"best_score_before"`
	BestScoreAfter    float64 `json:"best_score_after"`
	PreviousRank      *int    `json:"previous_rank,omitempty"`
	CurrentRank       int     `json:"current_rank"`
	TotalPoints       float64 `json:"total_points"`
	ImprovedBestScore bool    `json:"improved_best_score"`
}

type LeaderboardEntry struct {
	Rank                 int       `json:"rank"`
	UserID               string    `json:"user_id"`
	DisplayName          string    `json:"display_name"`
	IsCurrentUser        bool      `json:"is_current_user"`
	TotalPoints          float64   `json:"total_points"`
	CompletedExamSets    int       `json:"completed_exam_sets"`
	TotalDurationSeconds int64     `json:"total_duration_seconds"`
	ScoreAchievedAt      time.Time `json:"score_achieved_at"`
}

type CurrentUserSummary struct {
	Rank                 *int      `json:"rank,omitempty"`
	TotalPoints          float64   `json:"total_points"`
	CompletedExamSets    int       `json:"completed_exam_sets"`
	TotalDurationSeconds int64     `json:"total_duration_seconds"`
	ScoreAchievedAt      time.Time `json:"score_achieved_at"`
}

type Award struct {
	SeasonID  string    `json:"season_id"`
	TrackCode string    `json:"track_code"`
	Year      int       `json:"year"`
	Month     int       `json:"month"`
	Rank      int       `json:"rank"`
	Medal     string    `json:"medal"`
	AwardedAt time.Time `json:"awarded_at"`
}

type OverviewResponse struct {
	DefaultTrackCode  *string             `json:"default_track_code"`
	Season            *SeasonWindow       `json:"season"`
	ExamTrack         *ExamTrackRef       `json:"exam_track"`
	CurrentUser       *CurrentUserSummary `json:"current_user"`
	TopThree          []LeaderboardEntry  `json:"top_three"`
	NextOpportunities []ExamSetRef        `json:"next_opportunities"`
}

type HubResponse struct {
	Season            SeasonWindow        `json:"season"`
	ExamTrack         ExamTrackRef        `json:"exam_track"`
	CurrentUser       *CurrentUserSummary `json:"current_user"`
	TopThree          []LeaderboardEntry  `json:"top_three"`
	Leaderboard       []LeaderboardEntry  `json:"leaderboard"`
	NextOpportunities []ExamSetRef        `json:"next_opportunities"`
	Pagination        Pagination          `json:"pagination"`
}

type ListFilter struct {
	Page  int
	Limit int
}

type ExamSetContext struct {
	ID            uuid.UUID
	Code          string
	Title         string
	ExamTrackName string
	PassingScore  int
}

type ExamTrackContext struct {
	ID   uuid.UUID
	Code string
	Name string
}

func BangkokSeasonWindow(at time.Time) (SeasonWindow, error) {
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return SeasonWindow{}, err
	}

	local := at.In(location)
	startsAt := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	endsAt := startsAt.AddDate(0, 1, 0)

	return SeasonWindow{
		Year:     local.Year(),
		Month:    int(local.Month()),
		StartsAt: startsAt.UTC(),
		EndsAt:   endsAt.UTC(),
	}, nil
}

func AttemptWins(candidate, current ScoreCandidate) bool {
	if candidate.Points != current.Points {
		return candidate.Points > current.Points
	}
	if candidate.DurationSeconds != current.DurationSeconds {
		return candidate.DurationSeconds < current.DurationSeconds
	}
	return candidate.AchievedAt.Before(current.AchievedAt)
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
