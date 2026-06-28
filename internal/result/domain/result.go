package domain

import "time"

type WeakSubject struct {
	SubjectCode         string  `json:"subject_code,omitempty"`
	SubjectName         string  `json:"subject_name"`
	AverageScorePercent float64 `json:"average_score_percent"`
	Recommendation      string  `json:"recommendation,omitempty"`
}

type ScoreTrendPoint struct {
	AttemptID    string  `json:"attempt_id"`
	SubmittedAt  string  `json:"submitted_at"`
	ExamSetTitle string  `json:"exam_set_title"`
	ScorePercent float64 `json:"score_percent"`
	Passed       bool    `json:"passed"`
}

type SubjectPerformanceItem struct {
	SubjectCode    string  `json:"subject_code,omitempty"`
	SubjectName    string  `json:"subject_name"`
	ScorePercent   float64 `json:"score_percent"`
	TotalAttempts  int64   `json:"total_attempts,omitempty"`
	TotalQuestions int64   `json:"total_questions,omitempty"`
}

type ExamTrackRef struct {
	ID            string  `json:"id,omitempty"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	CoverImageURL *string `json:"cover_image_url,omitempty"`
}

type ExamSetRef struct {
	ID              string  `json:"id,omitempty"`
	Code            string  `json:"code"`
	Title           string  `json:"title"`
	CoverImageURL   *string `json:"cover_image_url,omitempty"`
	TotalQuestions  int     `json:"total_questions,omitempty"`
	DurationMinutes int     `json:"duration_minutes,omitempty"`
	PassingScore    int     `json:"passing_score,omitempty"`
}

type OverallSummary struct {
	TotalAttempts           int64        `json:"total_attempts"`
	CompletedExamSets       int64        `json:"completed_exam_sets"`
	CompletedExamTracks     int64        `json:"completed_exam_tracks"`
	AverageScorePercent     float64      `json:"average_score_percent"`
	BestScorePercent        float64      `json:"best_score_percent"`
	LatestScorePercent      float64      `json:"latest_score_percent"`
	PassedAttempts          int64        `json:"passed_attempts"`
	FailedAttempts          int64        `json:"failed_attempts"`
	PassRatePercent         float64      `json:"pass_rate_percent"`
	AverageDurationSeconds  float64      `json:"average_duration_seconds"`
	MostPracticedExamTrack  *ExamTrackRef            `json:"most_practiced_exam_track,omitempty"`
	WeakSubjects            []WeakSubject            `json:"weak_subjects"`
	ScoreTrend              []ScoreTrendPoint        `json:"score_trend"`
	SubjectPerformance      []SubjectPerformanceItem `json:"subject_performance"`
}

type ExamTrackSummaryItem struct {
	ExamTrack                ExamTrackRef  `json:"exam_track"`
	CompletedExamSets        int           `json:"completed_exam_sets"`
	TotalExamSets            int           `json:"total_exam_sets"`
	TotalAttempts            int64         `json:"total_attempts"`
	AverageBestScorePercent  float64       `json:"average_best_score_percent"`
	BestScorePercent         float64       `json:"best_score_percent"`
	LatestScorePercent       float64       `json:"latest_score_percent"`
	PassedExamSets           int           `json:"passed_exam_sets"`
	FailedExamSets           int           `json:"failed_exam_sets"`
	AverageDurationSeconds   float64       `json:"average_duration_seconds"`
	LastAttemptAt            *time.Time    `json:"last_attempt_at,omitempty"`
	WeakSubjects             []WeakSubject `json:"weak_subjects"`
}

type TrackSummaryStats struct {
	CompletedExamSets       int     `json:"completed_exam_sets"`
	TotalExamSets           int     `json:"total_exam_sets"`
	TotalAttempts           int64   `json:"total_attempts"`
	AverageBestScorePercent float64 `json:"average_best_score_percent"`
	BestScorePercent        float64 `json:"best_score_percent"`
	LatestScorePercent      float64 `json:"latest_score_percent"`
	PassedExamSets          int     `json:"passed_exam_sets"`
	FailedExamSets          int     `json:"failed_exam_sets"`
	AverageDurationSeconds  float64 `json:"average_duration_seconds"`
	ReadinessPercent        float64 `json:"readiness_percent"`
}

type ExamSetProgressItem struct {
	ExamSet            ExamSetRef `json:"exam_set"`
	AttemptCount       int        `json:"attempt_count"`
	LatestAttemptID    string     `json:"latest_attempt_id,omitempty"`
	LatestScorePercent float64    `json:"latest_score_percent"`
	BestAttemptID      string     `json:"best_attempt_id,omitempty"`
	BestScorePercent   float64    `json:"best_score_percent"`
	FirstScorePercent  float64    `json:"first_score_percent"`
	ImprovementPercent float64    `json:"improvement_percent"`
	Passed             bool       `json:"passed"`
	LastAttemptAt      *time.Time `json:"last_attempt_at,omitempty"`
}

type ExamTrackDetailResponse struct {
	ExamTrack        ExamTrackRef          `json:"exam_track"`
	Summary          TrackSummaryStats     `json:"summary"`
	ExamSets         []ExamSetProgressItem `json:"exam_sets"`
	WeaknessAnalysis []WeakSubject         `json:"weakness_analysis"`
}

type AttemptHistoryItem struct {
	AttemptID        string       `json:"attempt_id"`
	ExamTrack        ExamTrackRef `json:"exam_track"`
	ExamSet          ExamSetRef   `json:"exam_set"`
	AttemptNo        int          `json:"attempt_no"`
	Score            float64      `json:"score"`
	TotalScore       float64      `json:"total_score"`
	ScorePercent     float64      `json:"score_percent"`
	Passed           bool         `json:"passed"`
	CorrectCount     int          `json:"correct_count"`
	WrongCount       int          `json:"wrong_count"`
	UnansweredCount  int          `json:"unanswered_count"`
	DurationSeconds  int          `json:"duration_seconds"`
	Status           string       `json:"status"`
	StartedAt        time.Time    `json:"started_at"`
	SubmittedAt      *time.Time   `json:"submitted_at,omitempty"`
}

type AttemptHistoryFilter struct {
	ExamTrackCode string
	ExamSetCode   string
	Status        string // "passed" | "failed" | ""
	DateFrom      *time.Time
	DateTo        *time.Time
	Page          int
	Limit         int
}

type PaginatedAttempts struct {
	Items      []AttemptHistoryItem `json:"items"`
	Pagination Pagination           `json:"pagination"`
}

type Pagination struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

type ExamSetSummaryStats struct {
	AttemptCount           int     `json:"attempt_count"`
	FirstScorePercent      float64 `json:"first_score_percent"`
	LatestScorePercent     float64 `json:"latest_score_percent"`
	BestScorePercent       float64 `json:"best_score_percent"`
	ImprovementPercent     float64 `json:"improvement_percent"`
	Passed                 bool    `json:"passed"`
	AverageDurationSeconds float64 `json:"average_duration_seconds"`
}

type ExamSetAttemptItem struct {
	AttemptID       string     `json:"attempt_id"`
	AttemptNo       int        `json:"attempt_no"`
	ScorePercent    float64    `json:"score_percent"`
	Passed          bool       `json:"passed"`
	DurationSeconds int        `json:"duration_seconds"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
}

type ExamSetDetailResponse struct {
	ExamSet  ExamSetRef            `json:"exam_set"`
	Summary  ExamSetSummaryStats   `json:"summary"`
	Attempts []ExamSetAttemptItem  `json:"attempts"`
}
