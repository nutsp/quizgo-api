package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

type ExamSetContextRow struct {
	ID            uuid.UUID
	Code          string
	Title         string
	ExamTrackName string
	PassingScore  int
}

type ExamTrackContextRow struct {
	ID   uuid.UUID
	Code string
	Name string
}

type ExamSetLeaderboardRow struct {
	Rank            int
	UserID          uuid.UUID
	DisplayName     string
	Email           string
	Score           float64
	TotalScore      float64
	ScorePercent    float64
	PassingScore    int
	DurationSeconds *int
	SubmittedAt     *time.Time
}

type ExamSetUserRankRow struct {
	Rank            int
	ScorePercent    float64
	DurationSeconds *int
	SubmittedAt     *time.Time
}

type ExamTrackLeaderboardRow struct {
	Rank                int
	UserID              uuid.UUID
	DisplayName         string
	Email               string
	AverageScorePercent float64
	CompletedExamSets   int
	PassedExamSets      int
	PassRatePercent     float64
	LatestSubmittedAt   *time.Time
}

type ExamTrackUserRankRow struct {
	Rank                int
	AverageScorePercent float64
	CompletedExamSets   int
	PassedExamSets      int
	PassRatePercent     float64
}

type Repository interface {
	FindPublishedExamSetByCode(ctx context.Context, code string) (*ExamSetContextRow, error)
	FindActiveExamTrackByCode(ctx context.Context, code string) (*ExamTrackContextRow, error)
	CountExamSetLeaderboard(ctx context.Context, examSetID uuid.UUID) (int64, error)
	ListExamSetLeaderboard(ctx context.Context, examSetID uuid.UUID, offset, limit int) ([]ExamSetLeaderboardRow, error)
	GetExamSetUserRank(ctx context.Context, examSetID, userID uuid.UUID) (*ExamSetUserRankRow, error)
	CountExamTrackLeaderboard(ctx context.Context, trackID uuid.UUID) (int64, error)
	ListExamTrackLeaderboard(ctx context.Context, trackID uuid.UUID, offset, limit int) ([]ExamTrackLeaderboardRow, error)
	GetExamTrackUserRank(ctx context.Context, trackID, userID uuid.UUID) (*ExamTrackUserRankRow, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) FindPublishedExamSetByCode(ctx context.Context, code string) (*ExamSetContextRow, error) {
	var row ExamSetContextRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT es.id, es.code, es.title, et.name AS exam_track_name, es.passing_score
		FROM exam_sets es
		JOIN exam_tracks et ON et.id = es.exam_track_id
		WHERE es.code = ?
			AND es.status = ?
			AND es.is_active = true
			AND et.is_active = true
	`, code, examsetdomain.StatusPublished).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) FindActiveExamTrackByCode(ctx context.Context, code string) (*ExamTrackContextRow, error) {
	var row ExamTrackContextRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, code, name
		FROM exam_tracks
		WHERE code = ? AND is_active = true
	`, code).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

const examSetBestAttemptsCTE = `
	WITH best_attempts AS (
		SELECT DISTINCT ON (ea.user_id)
			ea.user_id,
			ea.score,
			ea.total_score,
			ea.score_percent,
			ea.duration_seconds,
			ea.submitted_at,
			es.passing_score
		FROM exam_attempts ea
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.exam_set_id = ?
			AND ea.status IN ('submitted', 'timeout')
			AND es.status = ?
			AND es.is_active = true
		ORDER BY ea.user_id, ea.score_percent DESC, ea.duration_seconds ASC NULLS LAST, ea.submitted_at ASC
	),
	ranked AS (
		SELECT
			ROW_NUMBER() OVER (
				ORDER BY score_percent DESC, duration_seconds ASC NULLS LAST, submitted_at ASC
			) AS rank,
			user_id,
			score,
			total_score,
			score_percent,
			passing_score,
			duration_seconds,
			submitted_at
		FROM best_attempts
	)
`

func (r *postgresRepository) CountExamSetLeaderboard(ctx context.Context, examSetID uuid.UUID) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT ea.user_id)
		FROM exam_attempts ea
		JOIN exam_sets es ON es.id = ea.exam_set_id
		WHERE ea.exam_set_id = ?
			AND ea.status IN ('submitted', 'timeout')
			AND es.status = ?
			AND es.is_active = true
	`, examSetID, examsetdomain.StatusPublished).Scan(&total).Error
	return total, err
}

func (r *postgresRepository) ListExamSetLeaderboard(ctx context.Context, examSetID uuid.UUID, offset, limit int) ([]ExamSetLeaderboardRow, error) {
	var rows []ExamSetLeaderboardRow
	err := r.db.WithContext(ctx).Raw(examSetBestAttemptsCTE+`
		SELECT
			r.rank,
			r.user_id,
			u.display_name,
			u.email,
			r.score,
			r.total_score,
			r.score_percent,
			r.passing_score,
			r.duration_seconds,
			r.submitted_at
		FROM ranked r
		JOIN users u ON u.id = r.user_id
		ORDER BY r.rank
		LIMIT ? OFFSET ?
	`, examSetID, examsetdomain.StatusPublished, limit, offset).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) GetExamSetUserRank(ctx context.Context, examSetID, userID uuid.UUID) (*ExamSetUserRankRow, error) {
	var row ExamSetUserRankRow
	err := r.db.WithContext(ctx).Raw(examSetBestAttemptsCTE+`
		SELECT rank, score_percent, duration_seconds, submitted_at
		FROM ranked
		WHERE user_id = ?
	`, examSetID, examsetdomain.StatusPublished, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Rank == 0 {
		return nil, nil
	}
	return &row, nil
}

const examTrackStatsCTE = `
	WITH active_sets AS (
		SELECT id, passing_score
		FROM exam_sets
		WHERE exam_track_id = ?
			AND status = ?
			AND is_active = true
	),
	best_per_set AS (
		SELECT DISTINCT ON (ea.user_id, ea.exam_set_id)
			ea.user_id,
			ea.exam_set_id,
			ea.score_percent,
			ea.submitted_at,
			es.passing_score
		FROM exam_attempts ea
		JOIN active_sets es ON es.id = ea.exam_set_id
		WHERE ea.exam_track_id = ?
			AND ea.status IN ('submitted', 'timeout')
		ORDER BY ea.user_id, ea.exam_set_id, ea.score_percent DESC, ea.duration_seconds ASC NULLS LAST, ea.submitted_at ASC
	),
	user_stats AS (
		SELECT
			user_id,
			AVG(score_percent) AS average_score_percent,
			COUNT(*)::int AS completed_exam_sets,
			COUNT(*) FILTER (WHERE score_percent >= passing_score)::int AS passed_exam_sets,
			MAX(submitted_at) AS latest_submitted_at
		FROM best_per_set
		GROUP BY user_id
		HAVING COUNT(*) >= 1
	),
	ranked AS (
		SELECT
			ROW_NUMBER() OVER (
				ORDER BY average_score_percent DESC,
					completed_exam_sets DESC,
					(passed_exam_sets::float / completed_exam_sets * 100) DESC,
					latest_submitted_at ASC
			) AS rank,
			user_id,
			average_score_percent,
			completed_exam_sets,
			passed_exam_sets,
			CASE WHEN completed_exam_sets > 0
				THEN (passed_exam_sets::float / completed_exam_sets * 100)
				ELSE 0 END AS pass_rate_percent,
			latest_submitted_at
		FROM user_stats
	)
`

func (r *postgresRepository) CountExamTrackLeaderboard(ctx context.Context, trackID uuid.UUID) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Raw(`
		WITH active_sets AS (
			SELECT id FROM exam_sets
			WHERE exam_track_id = ? AND status = ? AND is_active = true
		),
		best_per_set AS (
			SELECT DISTINCT ON (ea.user_id, ea.exam_set_id) ea.user_id
			FROM exam_attempts ea
			JOIN active_sets es ON es.id = ea.exam_set_id
			WHERE ea.exam_track_id = ?
				AND ea.status IN ('submitted', 'timeout')
			ORDER BY ea.user_id, ea.exam_set_id, ea.score_percent DESC
		)
		SELECT COUNT(DISTINCT user_id) FROM best_per_set
	`, trackID, examsetdomain.StatusPublished, trackID).Scan(&total).Error
	return total, err
}

func (r *postgresRepository) ListExamTrackLeaderboard(ctx context.Context, trackID uuid.UUID, offset, limit int) ([]ExamTrackLeaderboardRow, error) {
	var rows []ExamTrackLeaderboardRow
	err := r.db.WithContext(ctx).Raw(examTrackStatsCTE+`
		SELECT
			r.rank,
			r.user_id,
			u.display_name,
			u.email,
			r.average_score_percent,
			r.completed_exam_sets,
			r.passed_exam_sets,
			r.pass_rate_percent,
			r.latest_submitted_at
		FROM ranked r
		JOIN users u ON u.id = r.user_id
		ORDER BY r.rank
		LIMIT ? OFFSET ?
	`, trackID, examsetdomain.StatusPublished, trackID, limit, offset).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) GetExamTrackUserRank(ctx context.Context, trackID, userID uuid.UUID) (*ExamTrackUserRankRow, error) {
	var row ExamTrackUserRankRow
	err := r.db.WithContext(ctx).Raw(examTrackStatsCTE+`
		SELECT rank, average_score_percent, completed_exam_sets, passed_exam_sets, pass_rate_percent
		FROM ranked
		WHERE user_id = ?
	`, trackID, examsetdomain.StatusPublished, trackID, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Rank == 0 {
		return nil, nil
	}
	return &row, nil
}
