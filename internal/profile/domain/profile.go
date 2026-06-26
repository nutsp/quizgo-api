package domain

import "time"

type ProfileStats struct {
	TotalAttempts       int64   `json:"total_attempts"`
	CompletedExamSets   int64   `json:"completed_exam_sets"`
	AverageScorePercent float64 `json:"average_score_percent"`
	BestScorePercent    float64 `json:"best_score_percent"`
}

type ProfileResponse struct {
	ID                string        `json:"id"`
	Email             string        `json:"email"`
	DisplayName       string        `json:"display_name"`
	PublicDisplayName string        `json:"public_display_name"`
	Role              string        `json:"role"`
	CreatedAt         time.Time     `json:"created_at"`
	Stats             *ProfileStats `json:"stats,omitempty"`
}

type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

type UpdateProfileResponse struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	DisplayName       string `json:"display_name"`
	PublicDisplayName string `json:"public_display_name"`
}
