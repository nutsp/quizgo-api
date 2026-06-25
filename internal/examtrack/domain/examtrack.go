package domain

import (
	"time"

	"github.com/google/uuid"
)

type ExamTrack struct {
	ID             uuid.UUID
	Code           string
	Name           string
	Description    string
	CoverImageURL  *string
	TotalExamSets  int
	TotalQuestions int
	TotalAttempts  int
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ExamTrackSummary struct {
	ID             string  `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	CoverImageURL  *string `json:"cover_image_url,omitempty"`
	TotalExamSets  int     `json:"total_exam_sets"`
	TotalQuestions int     `json:"total_questions"`
	TotalAttempts  int     `json:"total_attempts"`
}

func (t *ExamTrack) ToSummary() ExamTrackSummary {
	return ExamTrackSummary{
		ID:             t.ID.String(),
		Code:           t.Code,
		Name:           t.Name,
		Description:    t.Description,
		CoverImageURL:  t.CoverImageURL,
		TotalExamSets:  t.TotalExamSets,
		TotalQuestions: t.TotalQuestions,
		TotalAttempts:  t.TotalAttempts,
	}
}

type ListFilter struct {
	Query      string
	Page       int
	Limit      int
	AccessType string
	Difficulty string
	Mode       string
}

type PaginatedExamSets struct {
	Items      []any `json:"items"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}
