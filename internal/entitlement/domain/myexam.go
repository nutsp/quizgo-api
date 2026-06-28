package domain

import (
	"time"

	"github.com/google/uuid"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

const (
	MyExamSourceSinglePurchase  = "single_purchase"
	MyExamSourceManualGrant     = "manual_grant"
	MyExamSourcePrivateGrant    = "private_grant"
	MyExamSourcePremiumActivity = "premium_activity"
	MyExamSourceFreeActivity    = "free_activity"

	// Legacy alias kept for backward compatibility in tests/clients.
	AccessSourceGranted = "granted"
)

type MyExamSummary struct {
	HasPremium            bool    `json:"has_premium"`
	PremiumExpiresAt      *string `json:"premium_expires_at,omitempty"`
	UnlockedExamSetCount  int     `json:"unlocked_exam_set_count"`
	PrivateExamSetCount   int     `json:"private_exam_set_count"`
	SinglePurchaseCount   int     `json:"single_purchase_count"`
	PremiumActivityCount  int     `json:"premium_activity_count"`
	GrantCount            int     `json:"grant_count"`
}

type MyExamEntitlement struct {
	ID        string  `json:"id,omitempty"`
	Source    string  `json:"source,omitempty"`
	StartsAt  string  `json:"starts_at,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"`
	Status    string  `json:"status,omitempty"`
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
	SourceLabel     string               `json:"source_label"`
	CanStart        bool                 `json:"can_start"`
	CoverImageURL   *string              `json:"cover_image_url,omitempty"`
	TotalQuestions  int                  `json:"total_questions"`
	DurationMinutes int                  `json:"duration_minutes"`
	Difficulty      string               `json:"difficulty,omitempty"`
	PassingScore    int                  `json:"passing_score,omitempty"`
	AllowSinglePurchase bool             `json:"allow_single_purchase,omitempty"`
	ExamTrack       *MyExamTrackRef      `json:"exam_track,omitempty"`
	Entitlement     *MyExamEntitlement   `json:"entitlement,omitempty"`
	LatestAttempt   *MyExamLatestAttempt `json:"latest_attempt,omitempty"`
}

type MyExamsResponse struct {
	Summary MyExamSummary `json:"summary"`
	Items   []MyExamItem  `json:"items"`
}

type AccessibleExamSetRow struct {
	ExamSet            examsetdomain.ExamSet
	Entitlement        Entitlement
	AttemptID          *uuid.UUID
	AttemptStatus      *string
	ScorePercent       *float64
	AttemptSubmittedAt *time.Time
}

type LatestAttemptRow struct {
	ExamSetID          uuid.UUID
	AttemptID          uuid.UUID
	Status             string
	ScorePercent       *float64
	SubmittedAt        *time.Time
	AccessSource       *string
	ExamSet            examsetdomain.ExamSet
}

type EntitlementExamSetRow struct {
	ExamSet     examsetdomain.ExamSet
	Entitlement Entitlement
}

var myExamSourcePriority = map[string]int{
	MyExamSourcePrivateGrant:    5,
	MyExamSourceManualGrant:     4,
	MyExamSourceSinglePurchase:  3,
	MyExamSourcePremiumActivity: 2,
	MyExamSourceFreeActivity:    1,
}

func ResolveEntitlementAccessSource(setAccessType, entitlementSource string) string {
	if setAccessType == examsetdomain.AccessPrivate {
		return MyExamSourcePrivateGrant
	}
	if entitlementSource == SourceManual {
		return MyExamSourceManualGrant
	}
	return MyExamSourceSinglePurchase
}

func ResolveActivityAccessSource(setAccessType, attemptAccessSource string) string {
	if setAccessType == examsetdomain.AccessFree {
		return MyExamSourceFreeActivity
	}
	if attemptAccessSource == AccessSourcePremium {
		return MyExamSourcePremiumActivity
	}
	if setAccessType == examsetdomain.AccessPremium {
		return MyExamSourcePremiumActivity
	}
	return MyExamSourceFreeActivity
}

func PickHigherPrioritySource(current, candidate string) string {
	if current == "" {
		return candidate
	}
	if myExamSourcePriority[candidate] > myExamSourcePriority[current] {
		return candidate
	}
	return current
}

func MyExamSourceLabel(source string, hasPremium bool) string {
	switch source {
	case MyExamSourceSinglePurchase:
		return "ซื้อรายชุด"
	case MyExamSourceManualGrant:
		return "ผู้ดูแลระบบมอบสิทธิ์"
	case MyExamSourcePrivateGrant:
		return "เฉพาะผู้ได้รับสิทธิ์"
	case MyExamSourcePremiumActivity:
		if hasPremium {
			return "ใช้งานผ่าน Premium"
		}
		return "เคยทำผ่าน Premium\nPremium หมดอายุแล้ว"
	case MyExamSourceFreeActivity:
		return "เคยทำข้อสอบฟรี"
	default:
		return ""
	}
}

func ShouldIncludeActivityRow(setAccessType string, attemptAccessSource *string) bool {
	switch setAccessType {
	case examsetdomain.AccessFree:
		return true
	case examsetdomain.AccessPremium:
		return true
	default:
		return true
	}
}
