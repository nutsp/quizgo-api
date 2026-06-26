package domain

import examsetdomain "virtual-exam-api/internal/examset/domain"

type HomeResponse struct {
	RecommendedExamTracks []ExamTrackItem                    `json:"recommended_exam_tracks"`
	PopularExamSets       []examsetdomain.ExamSetSummary       `json:"popular_exam_sets"`
	ContinueAttempt       *ContinueAttempt                     `json:"continue_attempt"`
	MyProgressSummary     *ProgressSummary                     `json:"my_progress_summary"`
}

type ExamTrackItem struct {
	ID             string  `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	CoverImageURL  *string `json:"cover_image_url,omitempty"`
	TotalExamSets  int     `json:"total_exam_sets"`
	TotalQuestions int     `json:"total_questions"`
}

type ExamTrackRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ContinueAttempt struct {
	AttemptID        string `json:"attempt_id"`
	ExamSetCode      string `json:"exam_set_code"`
	ExamSetTitle     string `json:"exam_set_title"`
	AnsweredCount    int    `json:"answered_count"`
	TotalQuestions   int    `json:"total_questions"`
	RemainingSeconds int    `json:"remaining_seconds"`
}

type ProgressSummary struct {
	AverageScorePercent float64 `json:"average_score_percent"`
	CompletedAttempts   int64   `json:"completed_attempts"`
	LatestWeakSubject   string  `json:"latest_weak_subject"`
}
