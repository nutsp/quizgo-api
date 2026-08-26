package domain

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

const (
	BlueprintStatusDraft    = "draft"
	BlueprintStatusReviewed = "reviewed"
)

var ErrInvalidBlueprint = errors.New("invalid exam blueprint")

type BlueprintSection struct {
	SubjectID     uuid.UUID  `json:"subject_id"`
	TagID         *uuid.UUID `json:"tag_id,omitempty"`
	WeightPercent float64    `json:"weight_percent"`
}

type Blueprint struct {
	Version         int                `json:"version"`
	Status          string             `json:"status"`
	QuestionCount   int                `json:"question_count"`
	DurationMinutes int                `json:"duration_minutes"`
	PassingScore    int                `json:"passing_score"`
	EffectiveDate   *time.Time         `json:"effective_date,omitempty"`
	ReviewedAt      *time.Time         `json:"reviewed_at,omitempty"`
	SourceNote      string             `json:"source_note,omitempty"`
	Sections        []BlueprintSection `json:"sections"`
}

func (b Blueprint) IsEmpty() bool {
	return b.Status == "" && b.QuestionCount == 0 && b.DurationMinutes == 0 && len(b.Sections) == 0
}

func (b *Blueprint) Normalize() {
	if b.Version < 1 {
		b.Version = 1
	}
	if b.Status == "" {
		b.Status = BlueprintStatusDraft
	}
	if b.Sections == nil {
		b.Sections = []BlueprintSection{}
	}
}

func (b Blueprint) ValidateForReview() error {
	if b.Version < 1 || b.Status != BlueprintStatusReviewed || b.QuestionCount < 1 || b.DurationMinutes < 1 || b.PassingScore < 0 || b.PassingScore > 100 || len(b.Sections) == 0 {
		return ErrInvalidBlueprint
	}
	total := 0.0
	seen := make(map[string]struct{}, len(b.Sections))
	for _, section := range b.Sections {
		if section.SubjectID == uuid.Nil || section.WeightPercent <= 0 || section.WeightPercent > 100 {
			return ErrInvalidBlueprint
		}
		key := section.SubjectID.String()
		if section.TagID != nil {
			key += ":" + section.TagID.String()
		}
		if _, exists := seen[key]; exists {
			return ErrInvalidBlueprint
		}
		seen[key] = struct{}{}
		total += section.WeightPercent
	}
	if math.Abs(total-100) > 0.01 {
		return ErrInvalidBlueprint
	}
	return nil
}
