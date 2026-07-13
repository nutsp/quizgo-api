package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	"virtual-exam-api/internal/leaderboard/domain"
)

type SeasonRow struct {
	ID          uuid.UUID
	ExamTrackID uuid.UUID
	Year        int
	Month       int
	StartsAt    time.Time
	EndsAt      time.Time
}

type BestScoreUpdate struct {
	Previous *domain.ScoreCandidate
	Current  domain.ScoreCandidate
	Improved bool
}

type EntryRow struct {
	TotalPoints          float64
	CompletedExamSets    int
	TotalDurationSeconds int64
	ScoreAchievedAt      time.Time
}

type UserRankRow struct {
	Rank        int
	TotalPoints float64
}

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
	EnsureSeason(ctx context.Context, examTrackID uuid.UUID, window domain.SeasonWindow) (*SeasonRow, error)
	EnsureSeasonForPublish(ctx context.Context, examTrackID, examSetID uuid.UUID, window domain.SeasonWindow) (*SeasonRow, error)
	JoinExamSet(ctx context.Context, seasonID, examSetID uuid.UUID, joinedAt time.Time) error
	StopExamSet(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error
	GetEligibleSeason(ctx context.Context, examSetID uuid.UUID, submittedAt time.Time) (*SeasonRow, error)
	UpsertBestScore(ctx context.Context, seasonID, userID, examSetID, attemptID uuid.UUID, candidate domain.ScoreCandidate) (*BestScoreUpdate, error)
	RebuildEntry(ctx context.Context, seasonID, userID uuid.UUID) (*EntryRow, error)
	GetUserRank(ctx context.Context, seasonID, userID uuid.UUID) (*UserRankRow, error)
	RecordProjectionFailure(ctx context.Context, attemptID uuid.UUID, projectionErr error) error

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

const insertSeasonSQL = `
	INSERT INTO leaderboard_seasons (
		id, exam_track_id, year, month, starts_at, ends_at, status
	)
	VALUES (?, ?, ?, ?, ?, ?, 'active')
	ON CONFLICT (exam_track_id, year, month) DO NOTHING
	RETURNING id, exam_track_id, year, month, starts_at, ends_at
`

const findSeasonSQL = `
	SELECT id, exam_track_id, year, month, starts_at, ends_at
	FROM leaderboard_seasons
	WHERE exam_track_id = ? AND year = ? AND month = ?
`

const enrollSeasonExamSetsSQL = `
	INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
	SELECT gen_random_uuid(), ?, id, ?
	FROM exam_sets
	WHERE exam_track_id = ?
		AND status = ?
		AND is_active = true
`

const enrollSeasonExamSetsForPublishSQL = enrollSeasonExamSetsSQL + `
		AND id <> ?
`

func (r *postgresRepository) EnsureSeason(ctx context.Context, examTrackID uuid.UUID, window domain.SeasonWindow) (*SeasonRow, error) {
	return r.ensureSeason(ctx, examTrackID, window, nil)
}

func (r *postgresRepository) EnsureSeasonForPublish(
	ctx context.Context,
	examTrackID, examSetID uuid.UUID,
	window domain.SeasonWindow,
) (*SeasonRow, error) {
	return r.ensureSeason(ctx, examTrackID, window, &examSetID)
}

func (r *postgresRepository) ensureSeason(
	ctx context.Context,
	examTrackID uuid.UUID,
	window domain.SeasonWindow,
	excludedExamSetID *uuid.UUID,
) (*SeasonRow, error) {
	var row SeasonRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Raw(
			insertSeasonSQL,
			uuid.New(), examTrackID, window.Year, window.Month, window.StartsAt, window.EndsAt,
		).Scan(&row)
		if insert.Error != nil {
			return insert.Error
		}
		if row.ID == uuid.Nil {
			return tx.Raw(findSeasonSQL, examTrackID, window.Year, window.Month).Scan(&row).Error
		}

		enrollmentSQL := enrollSeasonExamSetsSQL
		enrollmentArgs := []any{row.ID, row.StartsAt, examTrackID, examsetdomain.StatusPublished}
		if excludedExamSetID != nil {
			enrollmentSQL = enrollSeasonExamSetsForPublishSQL
			enrollmentArgs = append(enrollmentArgs, *excludedExamSetID)
		}
		return tx.Exec(enrollmentSQL, enrollmentArgs...).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

const acquireExamSetTransitionLockSQL = `
	SELECT pg_advisory_xact_lock(hashtextextended(CAST(? AS text), 0))
`

const findExactExamSetIntervalSQL = `
	SELECT EXISTS (
		SELECT 1
		FROM leaderboard_season_exam_sets
		WHERE season_id = ? AND exam_set_id = ? AND joined_at = ?
	)
`

const findOpenExamSetIntervalSQL = `
	SELECT id, joined_at
	FROM leaderboard_season_exam_sets
	WHERE season_id = ? AND exam_set_id = ? AND stopped_at IS NULL
	FOR UPDATE
`

const closeOpenExamSetIntervalSQL = `
	UPDATE leaderboard_season_exam_sets
	SET stopped_at = ?
	WHERE id = ? AND stopped_at IS NULL AND joined_at < ?
`

const insertSeasonExamSetIntervalSQL = `
	INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
	VALUES (?, ?, ?, ?)
`

const stopOpenExamSetIntervalsSQL = `
	UPDATE leaderboard_season_exam_sets
	SET stopped_at = ?
	WHERE exam_set_id = ?
		AND stopped_at IS NULL
		AND joined_at <= ?
`

func (r *postgresRepository) JoinExamSet(ctx context.Context, seasonID, examSetID uuid.UUID, joinedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}

		var exactIntervalExists bool
		if err := tx.Raw(
			findExactExamSetIntervalSQL,
			seasonID, examSetID, joinedAt,
		).Scan(&exactIntervalExists).Error; err != nil {
			return err
		}
		if exactIntervalExists {
			return nil
		}

		var openInterval struct {
			ID       uuid.UUID
			JoinedAt time.Time
		}
		if err := tx.Raw(
			findOpenExamSetIntervalSQL,
			seasonID, examSetID,
		).Scan(&openInterval).Error; err != nil {
			return err
		}
		if openInterval.ID != uuid.Nil {
			if !openInterval.JoinedAt.Before(joinedAt) {
				return nil
			}
			closed := tx.Exec(closeOpenExamSetIntervalSQL, joinedAt, openInterval.ID, joinedAt)
			if closed.Error != nil {
				return closed.Error
			}
			if closed.RowsAffected != 1 {
				return fmt.Errorf("close leaderboard enrollment interval: affected %d rows", closed.RowsAffected)
			}
		}

		return tx.Exec(
			insertSeasonExamSetIntervalSQL,
			uuid.New(), seasonID, examSetID, joinedAt,
		).Error
	})
}

func (r *postgresRepository) StopExamSet(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}
		return tx.Exec(stopOpenExamSetIntervalsSQL, stoppedAt, examSetID, stoppedAt).Error
	})
}

func (r *postgresRepository) GetEligibleSeason(ctx context.Context, examSetID uuid.UUID, submittedAt time.Time) (*SeasonRow, error) {
	var row SeasonRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT s.id, s.exam_track_id, s.year, s.month, s.starts_at, s.ends_at
		FROM leaderboard_seasons s
		WHERE s.status = 'active'
			AND ? >= s.starts_at
			AND ? < s.ends_at
			AND EXISTS (
				SELECT 1
				FROM leaderboard_season_exam_sets ses
				WHERE ses.season_id = s.id
					AND ses.exam_set_id = ?
					AND ? >= ses.joined_at
					AND (ses.stopped_at IS NULL OR ? < ses.stopped_at)
			)
		ORDER BY s.starts_at DESC
		LIMIT 1
	`, submittedAt, submittedAt, examSetID, submittedAt, submittedAt).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) UpsertBestScore(
	ctx context.Context,
	seasonID, userID, examSetID, attemptID uuid.UUID,
	candidate domain.ScoreCandidate,
) (*BestScoreUpdate, error) {
	var update BestScoreUpdate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Exec(`
			INSERT INTO leaderboard_scores (
				season_id, user_id, exam_set_id, attempt_id,
				points, duration_seconds, achieved_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (season_id, user_id, exam_set_id) DO NOTHING
		`, seasonID, userID, examSetID, attemptID, candidate.Points, candidate.DurationSeconds, candidate.AchievedAt)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 1 {
			update.Current = candidate
			update.Improved = true
			return nil
		}

		var current struct {
			Points          float64
			DurationSeconds int
			AchievedAt      time.Time
		}
		if err := tx.Raw(`
			SELECT points, duration_seconds, achieved_at
			FROM leaderboard_scores
			WHERE season_id = ? AND user_id = ? AND exam_set_id = ?
			FOR UPDATE
		`, seasonID, userID, examSetID).Scan(&current).Error; err != nil {
			return err
		}

		previous := domain.ScoreCandidate{
			Points:          current.Points,
			DurationSeconds: current.DurationSeconds,
			AchievedAt:      current.AchievedAt,
		}
		update.Previous = &previous
		update.Current = previous
		if !domain.AttemptWins(candidate, previous) {
			return nil
		}

		if err := tx.Exec(`
			UPDATE leaderboard_scores
			SET attempt_id = ?, points = ?, duration_seconds = ?, achieved_at = ?, updated_at = now()
			WHERE season_id = ? AND user_id = ? AND exam_set_id = ?
		`, attemptID, candidate.Points, candidate.DurationSeconds, candidate.AchievedAt, seasonID, userID, examSetID).Error; err != nil {
			return err
		}
		update.Current = candidate
		update.Improved = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &update, nil
}

func (r *postgresRepository) RebuildEntry(ctx context.Context, seasonID, userID uuid.UUID) (*EntryRow, error) {
	var entry EntryRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedAttempts []uuid.UUID
		if err := tx.Raw(`
			SELECT attempt_id
			FROM leaderboard_scores
			WHERE season_id = ? AND user_id = ?
			ORDER BY exam_set_id
			FOR UPDATE
		`, seasonID, userID).Scan(&lockedAttempts).Error; err != nil {
			return err
		}
		if len(lockedAttempts) == 0 {
			return tx.Exec(`
				DELETE FROM leaderboard_entries WHERE season_id = ? AND user_id = ?
			`, seasonID, userID).Error
		}

		if err := tx.Raw(`
			SELECT
				COALESCE(SUM(points), 0) AS total_points,
				COUNT(*)::int AS completed_exam_sets,
				COALESCE(SUM(duration_seconds), 0)::bigint AS total_duration_seconds,
				MAX(achieved_at) AS score_achieved_at
			FROM leaderboard_scores
			WHERE season_id = ? AND user_id = ?
		`, seasonID, userID).Scan(&entry).Error; err != nil {
			return err
		}

		return tx.Exec(`
			INSERT INTO leaderboard_entries (
				season_id, user_id, total_points, completed_exam_sets,
				total_duration_seconds, score_achieved_at
			)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (season_id, user_id) DO UPDATE SET
				total_points = EXCLUDED.total_points,
				completed_exam_sets = EXCLUDED.completed_exam_sets,
				total_duration_seconds = EXCLUDED.total_duration_seconds,
				score_achieved_at = EXCLUDED.score_achieved_at,
				updated_at = now()
		`, seasonID, userID, entry.TotalPoints, entry.CompletedExamSets, entry.TotalDurationSeconds, entry.ScoreAchievedAt).Error
	})
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

const monthlyRankWindowSQL = `
		total_points DESC,
		completed_exam_sets DESC,
		total_duration_seconds ASC,
		score_achieved_at ASC`

const monthlyRankedEntriesCTE = `
	WITH ranked AS (
		SELECT
			RANK() OVER (ORDER BY ` + monthlyRankWindowSQL + `) AS rank,
			user_id,
			total_points
		FROM leaderboard_entries
		WHERE season_id = ?
	),
	ordered AS (
		SELECT rank, user_id, total_points
		FROM ranked
		ORDER BY rank, user_id
	)
`

func (r *postgresRepository) GetUserRank(ctx context.Context, seasonID, userID uuid.UUID) (*UserRankRow, error) {
	var row UserRankRow
	err := r.db.WithContext(ctx).Raw(monthlyRankedEntriesCTE+`
		SELECT rank, total_points
		FROM ordered
		WHERE user_id = ?
	`, seasonID, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Rank == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) RecordProjectionFailure(ctx context.Context, attemptID uuid.UUID, projectionErr error) error {
	lastError := ""
	if projectionErr != nil {
		lastError = projectionErr.Error()
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO leaderboard_projection_failures (
			id, attempt_id, retry_count, last_error
		)
		VALUES (?, ?, 1, ?)
		ON CONFLICT (attempt_id) DO UPDATE SET
			retry_count = leaderboard_projection_failures.retry_count + 1,
			last_error = EXCLUDED.last_error,
			resolved_at = NULL,
			updated_at = now()
	`, uuid.New(), attemptID, lastError).Error
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
