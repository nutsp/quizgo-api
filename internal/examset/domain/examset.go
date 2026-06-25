package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	AccessFree    = "free"
	AccessPremium = "premium"

	ModePractice  = "practice"
	ModeMockExam  = "mock_exam"

	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

type ExamSet struct {
	ID              uuid.UUID
	ExamTrackID     uuid.UUID
	Code            string
	Title           string
	Description     string
	DurationMinutes int
	TotalQuestions  int
	PassingScore    int
	Difficulty      string
	AccessType      string
	Mode            string
	IsOfficial      bool
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExamTrack       *ExamTrackRef
}

type ExamTrackRef struct {
	Code string
	Name string
}

type ExamSetSummary struct {
	ID              string        `json:"id"`
	Code            string        `json:"code"`
	Title           string        `json:"title"`
	Description     string        `json:"description,omitempty"`
	DurationMinutes int           `json:"duration_minutes"`
	TotalQuestions  int           `json:"total_questions"`
	PassingScore    int           `json:"passing_score"`
	Difficulty      string        `json:"difficulty"`
	AccessType      string        `json:"access_type"`
	Mode            string        `json:"mode"`
	IsOfficial      bool          `json:"is_official"`
	IsActive        bool          `json:"is_active"`
	ExamTrack       *ExamTrackRef `json:"exam_track,omitempty"`
}

func (s ExamSet) ToSummary() ExamSetSummary {
	summary := ExamSetSummary{
		ID:              s.ID.String(),
		Code:            s.Code,
		Title:           s.Title,
		Description:     s.Description,
		DurationMinutes: s.DurationMinutes,
		TotalQuestions:  s.TotalQuestions,
		PassingScore:    s.PassingScore,
		Difficulty:      s.Difficulty,
		AccessType:      s.AccessType,
		Mode:            s.Mode,
		IsOfficial:      s.IsOfficial,
		IsActive:        s.IsActive,
	}
	if s.ExamTrack != nil {
		summary.ExamTrack = s.ExamTrack
	}
	return summary
}

type ListFilter struct {
	Query       string
	TrackCode   string
	TrackID     uuid.UUID
	AccessType  string
	Difficulty  string
	Mode        string
	Page        int
	Limit       int
	OnlyActive  bool
}

type PaginatedResult struct {
	Items      []ExamSetSummary `json:"items"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalItems int64            `json:"total_items"`
	TotalPages int              `json:"total_pages"`
}

type QuestionPreview struct {
	QuestionNo   int    `json:"question_no"`
	QuestionID   string `json:"question_id"`
	QuestionText string `json:"question_text"`
	SubjectName  string `json:"subject_name"`
	Difficulty   string `json:"difficulty"`
}

type QuestionsPreviewResponse struct {
	ExamSet   ExamSetSummary    `json:"exam_set"`
	Questions []QuestionPreview `json:"questions"`
}
