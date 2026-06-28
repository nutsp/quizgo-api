package domain

import (
	"time"

	"github.com/google/uuid"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

const (
	AccessSourceSinglePurchase = "single_purchase"
	AccessSourcePrivateGrant   = "private_grant"
	AccessSourceGranted        = "granted"
	AccessSourcePremium        = "premium"
)

type MyExamSummary struct {
	HasPremium           bool    `json:"has_premium"`
	PremiumExpiresAt     *string `json:"premium_expires_at,omitempty"`
	UnlockedExamSetCount int     `json:"unlocked_exam_set_count"`
	PrivateExamSetCount  int     `json:"private_exam_set_count"`
}

type MyExamEntitlement struct {
	ID        string     `json:"id"`
	Source    string     `json:"source"`
	StartsAt  string     `json:"starts_at"`
	ExpiresAt *string    `json:"expires_at,omitempty"`
	Status    string     `json:"status"`
}

type MyExamLatestAttempt struct {
	AttemptID    string   `json:"attempt_id"`
	Status       string   `json:"status"`
	ScorePercent *float64 `json:"score_percent,omitempty"`
	SubmittedAt  *string  `json:"submitted_at,omitempty"`
}

type MyExamTrackRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type MyExamItem struct {
	ID              string               `json:"id"`
	Code            string               `json:"code"`
	Title           string               `json:"title"`
	Description     string               `json:"description,omitempty"`
	AccessType      string               `json:"access_type"`
	AccessSource    string               `json:"access_source"`
	CoverImageURL   *string              `json:"cover_image_url,omitempty"`
	TotalQuestions  int                  `json:"total_questions"`
	DurationMinutes int                  `json:"duration_minutes"`
	Difficulty      string               `json:"difficulty,omitempty"`
	PassingScore    int                  `json:"passing_score,omitempty"`
	ExamTrack       *MyExamTrackRef      `json:"exam_track,omitempty"`
	Entitlement     MyExamEntitlement    `json:"entitlement"`
	LatestAttempt   *MyExamLatestAttempt `json:"latest_attempt,omitempty"`
}

type MyExamsResponse struct {
	Summary MyExamSummary `json:"summary"`
	Items   []MyExamItem  `json:"items"`
}

type AccessibleExamSetRow struct {
	ExamSet         examsetdomain.ExamSet
	Entitlement     Entitlement
	AttemptID       *uuid.UUID
	AttemptStatus   *string
	ScorePercent    *float64
	AttemptSubmittedAt *time.Time
}

func ResolveAccessSource(accessType string) string {
	switch accessType {
	case examsetdomain.AccessPrivate:
		return AccessSourcePrivateGrant
	case examsetdomain.AccessPaid:
		return AccessSourceSinglePurchase
	case examsetdomain.AccessPremium:
		return AccessSourceGranted
	default:
		return AccessSourceGranted
	}
}
