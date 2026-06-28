package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	attemptdomain "virtual-exam-api/internal/examattempt/domain"
	"virtual-exam-api/internal/result/domain"
)

type OverallStatsRow struct {
	TotalAttempts          int64
	CompletedExamSets      int64
	CompletedExamTracks    int64
	AverageScorePercent    *float64
	BestScorePercent       *float64
	LatestScorePercent     *float64
	PassedAttempts         int64
	FailedAttempts         int64
	AverageDurationSeconds *float64
}

type TrackPracticeRow struct {
	TrackID   uuid.UUID
	TrackCode string
	TrackName string
	AttemptCount int64
}

type BestAttemptRow struct {
	ExamSetID      uuid.UUID
	ExamTrackID    uuid.UUID
	ScorePercent   float64
	DurationSeconds *int
	SubmittedAt    *time.Time
	PassingScore   int
}

type AttemptRow struct {
	ID              uuid.UUID
	ExamTrackID     uuid.UUID
	ExamSetID       uuid.UUID
	TrackCode       string
	TrackName       string
	SetCode         string
	SetTitle        string
	SetCoverURL     *string
	SetPassingScore int
	Score           float64
	TotalScore      float64
	ScorePercent    float64
	CorrectCount    int
	WrongCount      int
	UnansweredCount int
	DurationSeconds *int
	Status          string
	StartedAt       time.Time
	SubmittedAt     *time.Time
}

type WeakSubjectRow struct {
	SubjectCode         string
	SubjectName         string
	AverageScorePercent float64
}

type ScoreTrendRow struct {
	AttemptID    uuid.UUID
	SubmittedAt  time.Time
	ExamSetTitle string
	ScorePercent float64
	PassingScore int
}

type SubjectPerformanceRow struct {
	SubjectCode         string
	SubjectName         string
	AverageScorePercent float64
	TotalAttempts       int64
	TotalQuestions      int64
}

type Repository interface {
	GetOverallStats(ctx context.Context, userID uuid.UUID) (*OverallStatsRow, error)
	GetMostPracticedTrack(ctx context.Context, userID uuid.UUID) (*TrackPracticeRow, error)
	ListWeakSubjects(ctx context.Context, userID uuid.UUID, trackID *uuid.UUID, limit int) ([]WeakSubjectRow, error)
	ListScoreTrend(ctx context.Context, userID uuid.UUID, limit int) ([]ScoreTrendRow, error)
	ListSubjectPerformance(ctx context.Context, userID uuid.UUID, limit int) ([]SubjectPerformanceRow, error)
	ListTracksWithAttempts(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	CountActiveExamSetsByTrack(ctx context.Context, trackID uuid.UUID) (int64, error)
	GetTrackBestAttempts(ctx context.Context, userID, trackID uuid.UUID) ([]BestAttemptRow, error)
	CountAttemptsByTrack(ctx context.Context, userID, trackID uuid.UUID) (int64, error)
	GetLatestAttemptScoreByTrack(ctx context.Context, userID, trackID uuid.UUID) (float64, bool, error)
	ListAttemptsByExamSet(ctx context.Context, userID, examSetID uuid.UUID) ([]AttemptRow, error)
	ListAttempts(ctx context.Context, userID uuid.UUID, filter domain.AttemptHistoryFilter) ([]AttemptRow, int64, error)
	FindTrackByCode(ctx context.Context, code string) (*TrackRow, error)
	FindTrackByID(ctx context.Context, id uuid.UUID) (*TrackRow, error)
	FindExamSetByCode(ctx context.Context, code string) (*ExamSetRow, error)
	ListActiveExamSetsByTrack(ctx context.Context, trackID uuid.UUID) ([]ExamSetRow, error)
}

type TrackRow struct {
	ID            uuid.UUID
	Code          string
	Name          string
	Description   string
	CoverImageURL *string
}

type ExamSetRow struct {
	ID              uuid.UUID
	ExamTrackID     uuid.UUID
	Code            string
	Title           string
	CoverImageURL   *string
	TotalQuestions  int
	DurationMinutes int
	PassingScore    int
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetOverallStats(ctx context.Context, userID uuid.UUID) (*OverallStatsRow, error) {
	var row OverallStatsRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS total_attempts,
			COUNT(DISTINCT ea.exam_set_id) AS completed_exam_sets,
			COUNT(DISTINCT ea.exam_track_id) AS completed_exam_tracks,
			AVG(ea.score_percent) AS average_score_percent,
			MAX(ea.score_percent) AS best_score_percent,
			(SELECT ea2.score_percent FROM exam_attempts ea2
			 WHERE ea2.user_id = ? AND ea2.status IN ('submitted', 'timeout')
			 ORDER BY ea2.submitted_at DESC NULLS LAST LIMIT 1) AS latest_score_percent,
			COUNT(*) FILTER (WHERE ea.score_percent >= es.passing_score) AS passed_attempts,
			COUNT(*) FILTER (WHERE ea.score_percent < es.passing_score) AS failed_attempts,
			AVG(ea.duration_seconds) AS average_duration_seconds
		FROM exam_attempts ea
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.user_id = ? AND ea.status IN ('submitted', 'timeout')
	`, userID, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *postgresRepository) GetMostPracticedTrack(ctx context.Context, userID uuid.UUID) (*TrackPracticeRow, error) {
	var row TrackPracticeRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT et.id AS track_id, et.code AS track_code, et.name AS track_name,
			COUNT(*) AS attempt_count
		FROM exam_attempts ea
		JOIN exam_tracks et ON et.id = ea.exam_track_id
		WHERE ea.user_id = ? AND ea.status IN ('submitted', 'timeout')
		GROUP BY et.id, et.code, et.name
		ORDER BY attempt_count DESC
		LIMIT 1
	`, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.TrackID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) ListScoreTrend(ctx context.Context, userID uuid.UUID, limit int) ([]ScoreTrendRow, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 20 {
		limit = 20
	}

	var rows []ScoreTrendRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			ea.id AS attempt_id,
			ea.submitted_at,
			es.title AS exam_set_title,
			ea.score_percent,
			es.passing_score
		FROM exam_attempts ea
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.user_id = ?
			AND ea.status IN ('submitted', 'timeout')
			AND ea.submitted_at IS NOT NULL
		ORDER BY ea.submitted_at DESC
		LIMIT ?
	`, userID, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

func (r *postgresRepository) ListSubjectPerformance(ctx context.Context, userID uuid.UUID, limit int) ([]SubjectPerformanceRow, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 8 {
		limit = 8
	}

	var rows []SubjectPerformanceRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			s.code AS subject_code,
			s.name AS subject_name,
			AVG(CASE WHEN ea_ans.is_correct = true THEN 100.0 ELSE 0.0 END) AS average_score_percent,
			COUNT(DISTINCT att.id) AS total_attempts,
			COUNT(*) AS total_questions
		FROM exam_attempts att
		JOIN exam_answers ea_ans ON ea_ans.attempt_id = att.id
		JOIN questions q ON q.id = ea_ans.question_id
		JOIN subjects s ON s.id = q.subject_id
		WHERE att.user_id = ? AND att.status IN ('submitted', 'timeout')
		GROUP BY s.code, s.name
		HAVING COUNT(*) > 0
		ORDER BY average_score_percent ASC
		LIMIT ?
	`, userID, limit).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListWeakSubjects(ctx context.Context, userID uuid.UUID, trackID *uuid.UUID, limit int) ([]WeakSubjectRow, error) {
	if limit < 1 {
		limit = 5
	}
	q := `
		SELECT s.code AS subject_code, s.name AS subject_name,
			AVG(CASE WHEN ea_ans.is_correct = true THEN 100.0 ELSE 0.0 END) AS average_score_percent
		FROM exam_attempts att
		JOIN exam_answers ea_ans ON ea_ans.attempt_id = att.id
		JOIN questions q ON q.id = ea_ans.question_id
		JOIN subjects s ON s.id = q.subject_id
		WHERE att.user_id = ? AND att.status IN ('submitted', 'timeout')
	`
	args := []any{userID}
	if trackID != nil {
		q += ` AND att.exam_track_id = ?`
		args = append(args, *trackID)
	}
	q += `
		GROUP BY s.code, s.name
		HAVING AVG(CASE WHEN ea_ans.is_correct = true THEN 100.0 ELSE 0.0 END) < 70
		ORDER BY average_score_percent ASC
		LIMIT ?
	`
	args = append(args, limit)

	var rows []WeakSubjectRow
	err := r.db.WithContext(ctx).Raw(q, args...).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListTracksWithAttempts(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT exam_track_id FROM exam_attempts
		WHERE user_id = ? AND status IN ('submitted', 'timeout')
		ORDER BY exam_track_id
	`, userID).Scan(&ids).Error
	return ids, err
}

func (r *postgresRepository) CountActiveExamSetsByTrack(ctx context.Context, trackID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM exam_sets WHERE exam_track_id = ? AND is_active = true
	`, trackID).Scan(&count).Error
	return count, err
}

func (r *postgresRepository) GetTrackBestAttempts(ctx context.Context, userID, trackID uuid.UUID) ([]BestAttemptRow, error) {
	var rows []BestAttemptRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (ea.exam_set_id)
			ea.exam_set_id,
			ea.exam_track_id,
			ea.score_percent,
			ea.duration_seconds,
			ea.submitted_at,
			es.passing_score
		FROM exam_attempts ea
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.user_id = ? AND ea.exam_track_id = ?
			AND ea.status IN ('submitted', 'timeout')
		ORDER BY ea.exam_set_id, ea.score_percent DESC, ea.duration_seconds ASC NULLS LAST
	`, userID, trackID).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) CountAttemptsByTrack(ctx context.Context, userID, trackID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&attemptModel{}).
		Where("user_id = ? AND exam_track_id = ? AND status IN ?",
			userID, trackID, []string{attemptdomain.StatusSubmitted, attemptdomain.StatusTimeout}).
		Count(&count).Error
	return count, err
}

func (r *postgresRepository) GetLatestAttemptScoreByTrack(ctx context.Context, userID, trackID uuid.UUID) (float64, bool, error) {
	var score *float64
	err := r.db.WithContext(ctx).Raw(`
		SELECT score_percent FROM exam_attempts
		WHERE user_id = ? AND exam_track_id = ? AND status IN ('submitted', 'timeout')
		ORDER BY submitted_at DESC NULLS LAST
		LIMIT 1
	`, userID, trackID).Scan(&score).Error
	if err != nil {
		return 0, false, err
	}
	if score == nil {
		return 0, false, nil
	}
	return *score, true, nil
}

func (r *postgresRepository) ListAttemptsByExamSet(ctx context.Context, userID, examSetID uuid.UUID) ([]AttemptRow, error) {
	var rows []AttemptRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			ea.id,
			ea.exam_track_id,
			ea.exam_set_id,
			et.code AS track_code,
			et.name AS track_name,
			es.code AS set_code,
			es.title AS set_title,
			es.cover_image_url AS set_cover_url,
			es.passing_score AS set_passing_score,
			ea.score,
			ea.total_score,
			ea.score_percent,
			ea.correct_count,
			ea.wrong_count,
			ea.unanswered_count,
			ea.duration_seconds,
			ea.status,
			ea.started_at,
			ea.submitted_at
		FROM exam_attempts ea
		JOIN exam_tracks et ON et.id = ea.exam_track_id
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.user_id = ? AND ea.exam_set_id = ?
			AND ea.status IN ('submitted', 'timeout')
		ORDER BY ea.submitted_at ASC NULLS LAST
	`, userID, examSetID).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListAttempts(ctx context.Context, userID uuid.UUID, filter domain.AttemptHistoryFilter) ([]AttemptRow, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	base := `
		FROM exam_attempts ea
		JOIN exam_tracks et ON et.id = ea.exam_track_id
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.user_id = ? AND ea.status IN ('submitted', 'timeout')
	`
	args := []any{userID}

	if filter.ExamTrackCode != "" {
		base += ` AND et.code = ?`
		args = append(args, filter.ExamTrackCode)
	}
	if filter.ExamSetCode != "" {
		base += ` AND es.code = ?`
		args = append(args, filter.ExamSetCode)
	}
	if filter.Status == "passed" {
		base += ` AND ea.score_percent >= es.passing_score`
	} else if filter.Status == "failed" {
		base += ` AND ea.score_percent < es.passing_score`
	}
	if filter.DateFrom != nil {
		base += ` AND ea.submitted_at >= ?`
		args = append(args, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		base += ` AND ea.submitted_at <= ?`
		args = append(args, *filter.DateTo)
	}

	var total int64
	if err := r.db.WithContext(ctx).Raw(`SELECT COUNT(*) `+base, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	selectQ := `
		SELECT
			ea.id,
			ea.exam_track_id,
			ea.exam_set_id,
			et.code AS track_code,
			et.name AS track_name,
			es.code AS set_code,
			es.title AS set_title,
			es.cover_image_url AS set_cover_url,
			es.passing_score AS set_passing_score,
			ea.score,
			ea.total_score,
			ea.score_percent,
			ea.correct_count,
			ea.wrong_count,
			ea.unanswered_count,
			ea.duration_seconds,
			ea.status,
			ea.started_at,
			ea.submitted_at
	` + base + ` ORDER BY ea.submitted_at DESC NULLS LAST LIMIT ? OFFSET ?`

	queryArgs := append(append([]any{}, args...), limit, offset)
	var rows []AttemptRow
	err := r.db.WithContext(ctx).Raw(selectQ, queryArgs...).Scan(&rows).Error
	return rows, total, err
}

func (r *postgresRepository) FindTrackByCode(ctx context.Context, code string) (*TrackRow, error) {
	var row TrackRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, code, name, description, cover_image_url
		FROM exam_tracks WHERE code = ? AND is_active = true
	`, code).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) FindTrackByID(ctx context.Context, id uuid.UUID) (*TrackRow, error) {
	var row TrackRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, code, name, description, cover_image_url
		FROM exam_tracks WHERE id = ?
	`, id).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) FindExamSetByCode(ctx context.Context, code string) (*ExamSetRow, error) {
	var row ExamSetRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, exam_track_id, code, title, cover_image_url,
			total_questions, duration_minutes, passing_score
		FROM exam_sets WHERE code = ?
	`, code).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) ListActiveExamSetsByTrack(ctx context.Context, trackID uuid.UUID) ([]ExamSetRow, error) {
	var rows []ExamSetRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, exam_track_id, code, title, cover_image_url,
			total_questions, duration_minutes, passing_score
		FROM exam_sets
		WHERE exam_track_id = ? AND is_active = true
		ORDER BY created_at ASC
	`, trackID).Scan(&rows).Error
	return rows, err
}

// attemptModel is a minimal GORM model for count queries.
type attemptModel struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

func (attemptModel) TableName() string { return "exam_attempts" }
