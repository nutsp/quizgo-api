package domain

import (
	"time"

	"github.com/google/uuid"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	leaderboarddomain "virtual-exam-api/internal/leaderboard/domain"
	settingsdomain "virtual-exam-api/internal/settings/domain"
)

const (
	StatusInProgress = "in_progress"
	StatusSubmitted  = "submitted"
	StatusTimeout    = "timeout"
	StatusCancelled  = "cancelled"

	TimingModeCountdown = "countdown"
	TimingModeElapsed   = "elapsed"
)

func NormalizeTimingMode(mode string) string {
	if mode == TimingModeElapsed {
		return TimingModeElapsed
	}
	return TimingModeCountdown
}

func usesCountdownDeadline(mode string) bool {
	return NormalizeTimingMode(mode) == TimingModeCountdown
}

func UsesCountdownDeadline(mode string) bool {
	return usesCountdownDeadline(mode)
}

type ExamAttempt struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	ExamTrackID         uuid.UUID
	ExamSetID           uuid.UUID
	Status              string
	TimingMode          string
	StartedAt           time.Time
	SubmittedAt         *time.Time
	ExpiresAt           time.Time
	AccessSource        *string
	AccessEntitlementID *uuid.UUID
	AccessGrantedAt     *time.Time
	AccessExpiresAt     *time.Time
	DurationSeconds     *int
	Score               float64
	TotalScore          float64
	ScorePercent        float64
	CorrectCount        int
	WrongCount          int
	UnansweredCount     int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ExamSet             *ExamSetRef
	ExamTrack           *ExamTrackRef
}

type ExamSetRef struct {
	Code              string                                `json:"code"`
	Title             string                                `json:"title"`
	DurationMinutes   int                                   `json:"duration_minutes"`
	TotalQuestions    int                                   `json:"total_questions"`
	PassingScore      int                                   `json:"passing_score,omitempty"`
	AccessType        string                                `json:"access_type,omitempty"`
	Access            *ResultAccessInfo                     `json:"access,omitempty"`
	AnswerSheetLayout examsetdomain.AnswerSheetLayoutConfig `json:"answer_sheet_layout"`
}

type ResultAccessInfo struct {
	HasPremium bool `json:"has_premium"`
}

type ExamTrackRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ExamAnswer struct {
	ID                uuid.UUID
	AttemptID         uuid.UUID
	QuestionID        uuid.UUID
	QuestionNo        int
	SelectedChoiceKey *string
	IsCorrect         *bool
	AnsweredAt        *time.Time
}

type StartAttemptResponse struct {
	AttemptID   string                                `json:"attempt_id"`
	TimingMode  string                                `json:"timing_mode"`
	ExamSet     ExamSetRef                            `json:"exam_set"`
	OMRSettings settingsdomain.OMRAnswerSheetSettings `json:"omr_settings"`
	StartedAt   time.Time                             `json:"started_at"`
	ExpiresAt   time.Time                             `json:"expires_at"`
	Questions   []QuestionForExam                     `json:"questions"`
	Answers     map[int]string                        `json:"answers"`
}

type StartAttemptRequest struct {
	TimingMode string `json:"timing_mode"`
}

type QuestionForExam struct {
	QuestionNo       int            `json:"question_no"`
	QuestionID       string         `json:"question_id"`
	QuestionText     string         `json:"question_text"`
	ContentFormat    string         `json:"content_format,omitempty"`
	QuestionImageURL *string        `json:"question_image_url,omitempty"`
	Choices          []ChoicePublic `json:"choices"`
}

type ChoicePublic struct {
	ChoiceKey      string  `json:"choice_key"`
	ChoiceLabel    string  `json:"choice_label"`
	ChoiceText     string  `json:"choice_text"`
	ContentFormat  string  `json:"content_format,omitempty"`
	ChoiceImageURL *string `json:"choice_image_url,omitempty"`
}

type GetAttemptResponse struct {
	AttemptID        string                                `json:"attempt_id"`
	Status           string                                `json:"status"`
	TimingMode       string                                `json:"timing_mode"`
	ExamSet          ExamSetRef                            `json:"exam_set"`
	OMRSettings      settingsdomain.OMRAnswerSheetSettings `json:"omr_settings"`
	StartedAt        time.Time                             `json:"started_at"`
	ExpiresAt        time.Time                             `json:"expires_at"`
	RemainingSeconds int                                   `json:"remaining_seconds"`
	ElapsedSeconds   int                                   `json:"elapsed_seconds"`
	Questions        []QuestionForExam                     `json:"questions"`
	Answers          map[int]string                        `json:"answers"`
	AnsweredCount    int                                   `json:"answered_count"`
	UnansweredCount  int                                   `json:"unanswered_count"`
}

type SaveAnswerRequest struct {
	SelectedChoiceKey string `json:"selected_choice_key" validate:"required"`
}

type SaveAnswerResponse struct {
	QuestionNo        int    `json:"question_no"`
	SelectedChoiceKey string `json:"selected_choice_key"`
	AnsweredCount     int    `json:"answered_count"`
	UnansweredCount   int    `json:"unanswered_count"`
	MarkedCount       int    `json:"marked_count,omitempty"`
}

type SubmitResponse struct {
	AttemptID         string                              `json:"attempt_id"`
	Status            string                              `json:"status"`
	Score             float64                             `json:"score"`
	TotalScore        float64                             `json:"total_score"`
	ScorePercent      float64                             `json:"score_percent"`
	CorrectCount      int                                 `json:"correct_count"`
	WrongCount        int                                 `json:"wrong_count"`
	UnansweredCount   int                                 `json:"unanswered_count"`
	DurationSeconds   int                                 `json:"duration_seconds"`
	Passed            bool                                `json:"passed"`
	CompetitionUpdate *leaderboarddomain.ProjectionUpdate `json:"competition_update,omitempty"`
}

type SubjectBreakdown struct {
	SubjectName  string  `json:"subject_name"`
	Correct      int     `json:"correct"`
	Wrong        int     `json:"wrong"`
	Unanswered   int     `json:"unanswered"`
	Total        int     `json:"total"`
	ScorePercent float64 `json:"score_percent"`
}

type WeaknessAnalysisItem struct {
	SubjectName    string  `json:"subject_name"`
	ScorePercent   float64 `json:"score_percent"`
	Recommendation string  `json:"recommendation"`
}

type ResultSummary struct {
	Status          string     `json:"status"`
	Score           float64    `json:"score"`
	TotalScore      float64    `json:"total_score"`
	ScorePercent    float64    `json:"score_percent"`
	Passed          bool       `json:"passed"`
	CorrectCount    int        `json:"correct_count"`
	WrongCount      int        `json:"wrong_count"`
	UnansweredCount int        `json:"unanswered_count"`
	DurationSeconds int        `json:"duration_seconds"`
	StartedAt       time.Time  `json:"started_at"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
}

type ResultResponse struct {
	AttemptID        string                 `json:"attempt_id"`
	ExamSet          ExamSetRef             `json:"exam_set"`
	ExamTrack        ExamTrackRef           `json:"exam_track"`
	Summary          ResultSummary          `json:"summary"`
	SubjectBreakdown []SubjectBreakdown     `json:"subject_breakdown"`
	WeaknessAnalysis []WeaknessAnalysisItem `json:"weakness_analysis"`
}

type ReviewResponse struct {
	AttemptID string              `json:"attempt_id"`
	ExamSet   ExamSetRef          `json:"exam_set"`
	Questions []QuestionForReview `json:"questions"`
}

type ReviewChoice struct {
	ChoiceKey      string  `json:"choice_key"`
	ChoiceLabel    string  `json:"choice_label"`
	ChoiceText     string  `json:"choice_text"`
	ContentFormat  string  `json:"content_format,omitempty"`
	ChoiceImageURL *string `json:"choice_image_url,omitempty"`
	IsSelected     bool    `json:"is_selected"`
	IsCorrect      bool    `json:"is_correct"`
}

type QuestionForReview struct {
	QuestionNo          int            `json:"question_no"`
	QuestionID          string         `json:"question_id"`
	QuestionText        string         `json:"question_text"`
	ContentFormat       string         `json:"content_format,omitempty"`
	QuestionImageURL    *string        `json:"question_image_url,omitempty"`
	Choices             []ReviewChoice `json:"choices"`
	SelectedChoiceKey   *string        `json:"selected_choice_key"`
	CorrectChoiceKey    string         `json:"correct_choice_key"`
	IsCorrect           bool           `json:"is_correct"`
	IsUnanswered        bool           `json:"is_unanswered"`
	Explanation         string         `json:"explanation"`
	ExplanationImageURL *string        `json:"explanation_image_url,omitempty"`
	Subject             string         `json:"subject"`
	Tags                []ReviewTagRef `json:"tags,omitempty"`
}

type ReviewTagRef struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type ContinueAttempt struct {
	AttemptID        string    `json:"attempt_id"`
	ExamSetCode      string    `json:"exam_set_code"`
	ExamSetTitle     string    `json:"exam_set_title"`
	AnsweredCount    int       `json:"answered_count"`
	TotalQuestions   int       `json:"total_questions"`
	RemainingSeconds int       `json:"remaining_seconds"`
	ElapsedSeconds   int       `json:"elapsed_seconds"`
	TimingMode       string    `json:"timing_mode"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type LatestAttemptSummary struct {
	ExamSetID    uuid.UUID
	AttemptID    uuid.UUID
	Status       string
	ScorePercent *float64
	SubmittedAt  *time.Time
	AccessSource *string
	StartedAt    time.Time
	ExpiresAt    time.Time
	TimingMode   string
}

type UserExamActivity struct {
	HasSubmittedAttempts     bool
	LatestSubmittedAttemptID *uuid.UUID
	LatestAttemptStatus      *string
	LatestScorePercent       *float64
}
