package repository

import (
	"context"
	"database/sql"
	"errors"
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
	Status      string
	FinalizedAt *time.Time
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

type BestScoreProjection struct {
	Season       *SeasonRow
	PreviousRank *UserRankRow
	ScoreUpdate  *BestScoreUpdate
	Entry        *EntryRow
	CurrentRank  *UserRankRow
}

var (
	ErrLifecycleStatePending = errors.New("leaderboard lifecycle state is pending")
	projectionTxOptions      = &sql.TxOptions{Isolation: sql.LevelReadCommitted}
)

var reconcileLifecycleSchemaSQL = []string{
	`CREATE EXTENSION IF NOT EXISTS btree_gist`,
	`DO $$
	BEGIN
		IF NOT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = 'leaderboard_season_exam_sets_no_overlap'
				AND conrelid = 'leaderboard_season_exam_sets'::regclass
		) THEN
			ALTER TABLE leaderboard_season_exam_sets
			ADD CONSTRAINT leaderboard_season_exam_sets_no_overlap
			EXCLUDE USING gist (
				season_id WITH =,
				exam_set_id WITH =,
				tstzrange(joined_at, stopped_at, '[)') WITH &&
			);
		END IF;
	END $$`,
}

// ReconcileLifecycleSchema restores constraints that GORM cannot express.
func ReconcileLifecycleSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range reconcileLifecycleSchemaSQL {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
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

type SeasonLeaderboardRow struct {
	Rank                 int
	UserID               uuid.UUID
	DisplayName          string
	Email                string
	TotalPoints          float64
	CompletedExamSets    int
	TotalDurationSeconds int64
	ScoreAchievedAt      time.Time
}

type SeasonUserSummaryRow struct {
	Rank                 int
	UserID               uuid.UUID
	TotalPoints          float64
	CompletedExamSets    int
	TotalDurationSeconds int64
	ScoreAchievedAt      time.Time
}

type NextOpportunityRow struct {
	Code          string
	Title         string
	ExamTrackName string
}

type AwardRow struct {
	SeasonID  uuid.UUID
	TrackCode string
	Year      int
	Month     int
	Rank      int
	AwardedAt time.Time
}

type FinalizationResult struct {
	SeasonID    uuid.UUID
	Finalized   bool
	FinalizedAt time.Time
	AwardCount  int64
}

type Repository interface {
	EnsureSeason(ctx context.Context, examTrackID uuid.UUID, window domain.SeasonWindow) (*SeasonRow, error)
	EnsureSeasonForPublish(ctx context.Context, examTrackID, examSetID uuid.UUID, window domain.SeasonWindow) (*SeasonRow, error)
	PublishExamSet(ctx context.Context, examTrackID, examSetID uuid.UUID, window domain.SeasonWindow, publishedAt time.Time) (*SeasonRow, error)
	JoinExamSet(ctx context.Context, seasonID, examSetID uuid.UUID, joinedAt time.Time) error
	StopExamSet(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error
	GetEligibleSeason(ctx context.Context, examSetID uuid.UUID, submittedAt time.Time) (*SeasonRow, error)
	ProjectBestScore(ctx context.Context, userID, examSetID, attemptID uuid.UUID, submittedAt time.Time, candidate domain.ScoreCandidate) (*BestScoreProjection, error)
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

	FindMostRecentAttemptedTrack(ctx context.Context, userID uuid.UUID) (*ExamTrackContextRow, error)
	FindSeason(ctx context.Context, trackID uuid.UUID, year, month int) (*SeasonRow, error)
	CountSeasonLeaderboard(ctx context.Context, seasonID uuid.UUID) (int64, error)
	ListSeasonLeaderboard(ctx context.Context, seasonID uuid.UUID, offset, limit int) ([]SeasonLeaderboardRow, error)
	ListSeasonTopThree(ctx context.Context, seasonID uuid.UUID) ([]SeasonLeaderboardRow, error)
	ListSeasonLeaderboardAroundUser(ctx context.Context, seasonID, userID uuid.UUID, above, below int) ([]SeasonLeaderboardRow, error)
	GetSeasonUserSummary(ctx context.Context, seasonID, userID uuid.UUID) (*SeasonUserSummaryRow, error)
	ListNextOpportunities(ctx context.Context, seasonID, userID uuid.UUID) ([]NextOpportunityRow, error)
	ListAwards(ctx context.Context, userID uuid.UUID) ([]AwardRow, error)
	ListDueSeasons(ctx context.Context, at time.Time) ([]SeasonRow, error)
	FinalizeSeason(ctx context.Context, seasonID uuid.UUID, finalizedAt time.Time) (*FinalizationResult, error)
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
	RETURNING id, exam_track_id, year, month, starts_at, ends_at, status, finalized_at
`

const findSeasonSQL = `
	SELECT id, exam_track_id, year, month, starts_at, ends_at, status, finalized_at
	FROM leaderboard_seasons
	WHERE exam_track_id = ? AND year = ? AND month = ?
`

const listRolloverExamSetCandidatesSQL = `
	SELECT es.id
	FROM exam_sets es
	WHERE es.exam_track_id = ?
		AND es.status = ?
		AND es.is_active = true
		AND (?::uuid IS NULL OR es.id <> ?)
	ORDER BY es.id
`

const findRolloverExamSetStateSQL = `
	SELECT id, exam_track_id, status, is_active, published_at
	FROM exam_sets
	WHERE id = ?
		AND exam_track_id = ?
		AND status = ?
		AND is_active = true
	FOR SHARE
`

const acquireSeasonLifecycleLockSQL = `
	SELECT pg_advisory_xact_lock(
		hashtextextended(
			CAST(? AS text) || ':' || CAST(CAST(? AS integer) AS text) || ':' ||
				CAST(CAST(? AS integer) AS text),
			2
		)
	)
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

func (r *postgresRepository) PublishExamSet(
	ctx context.Context,
	examTrackID, examSetID uuid.UUID,
	_ domain.SeasonWindow,
	publishedAt time.Time,
) (*SeasonRow, error) {
	state, err := loadExamSetPublicationState(r.db.WithContext(ctx), examSetID, false)
	if err != nil {
		return nil, err
	}
	if err := validateExamSetPublishState(state, examTrackID); err != nil {
		return nil, err
	}
	eventAt, err := persistedPublicationEventTime(state, publishedAt)
	if err != nil {
		return nil, err
	}
	window, err := domain.BangkokSeasonWindow(eventAt)
	if err != nil {
		return nil, err
	}

	var row SeasonRow
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			acquireSeasonLifecycleLockSQL,
			examTrackID,
			window.Year,
			window.Month,
		).Error; err != nil {
			return err
		}
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}
		lockedState, err := loadExamSetPublicationState(tx, examSetID, true)
		if err != nil {
			return err
		}
		if err := validateExamSetPublishState(lockedState, examTrackID); err != nil {
			return err
		}
		lockedEventAt, err := persistedPublicationEventTime(lockedState, publishedAt)
		if err != nil {
			return err
		}
		lockedWindow, err := domain.BangkokSeasonWindow(lockedEventAt)
		if err != nil {
			return err
		}
		if lockedWindow.Year != window.Year || lockedWindow.Month != window.Month {
			return fmt.Errorf("%w: exam set %s publication activation changed", ErrLifecycleStatePending, examSetID)
		}
		if err := r.ensureSeasonInTransaction(tx, examTrackID, window, &examSetID, &row); err != nil {
			return err
		}
		return publishExamSetInterval(tx, row.ID, examSetID, lockedEventAt)
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// The lifecycle caller passes the persisted activation event time. A timestamp
// later than the latest persisted activation is a retry clock and is clamped;
// an earlier timestamp remains an exact historical event for ordered replay.
func persistedPublicationEventTime(state *examSetPublicationStateRow, eventAt time.Time) (time.Time, error) {
	if state.PublishedAt == nil {
		return time.Time{}, fmt.Errorf(
			"%w: exam set %s has no persisted publication timestamp",
			ErrLifecycleStatePending,
			state.ID,
		)
	}
	if eventAt.IsZero() || eventAt.After(*state.PublishedAt) {
		return *state.PublishedAt, nil
	}
	return eventAt, nil
}

func (r *postgresRepository) ensureSeason(
	ctx context.Context,
	examTrackID uuid.UUID,
	window domain.SeasonWindow,
	excludedExamSetID *uuid.UUID,
) (*SeasonRow, error) {
	var row SeasonRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			acquireSeasonLifecycleLockSQL,
			examTrackID,
			window.Year,
			window.Month,
		).Error; err != nil {
			return err
		}
		return r.ensureSeasonInTransaction(tx, examTrackID, window, excludedExamSetID, &row)
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *postgresRepository) ensureSeasonInTransaction(
	tx *gorm.DB,
	examTrackID uuid.UUID,
	window domain.SeasonWindow,
	excludedExamSetID *uuid.UUID,
	row *SeasonRow,
) error {
	insert := tx.Raw(
		insertSeasonSQL,
		uuid.New(), examTrackID, window.Year, window.Month, window.StartsAt, window.EndsAt,
	).Scan(row)
	if insert.Error != nil {
		return insert.Error
	}
	if row.ID == uuid.Nil {
		return tx.Raw(findSeasonSQL, examTrackID, window.Year, window.Month).Scan(row).Error
	}
	return r.enrollSeasonExamSets(tx, *row, excludedExamSetID)
}

func (r *postgresRepository) enrollSeasonExamSets(
	tx *gorm.DB,
	season SeasonRow,
	excludedExamSetID *uuid.UUID,
) error {
	var excluded any
	if excludedExamSetID != nil {
		excluded = *excludedExamSetID
	}
	var candidateIDs []uuid.UUID
	if err := tx.Raw(
		listRolloverExamSetCandidatesSQL,
		season.ExamTrackID,
		examsetdomain.StatusPublished,
		excluded,
		excluded,
	).Scan(&candidateIDs).Error; err != nil {
		return err
	}

	for _, examSetID := range candidateIDs {
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}

		joinedAt, eligible, err := effectiveSeasonJoinTime(tx, season, examSetID)
		if err != nil {
			return err
		}
		if !eligible || !joinedAt.Before(season.EndsAt) {
			continue
		}
		if err := publishExamSetInterval(tx, season.ID, examSetID, joinedAt); err != nil {
			return err
		}
	}
	return nil
}

func effectiveSeasonJoinTime(
	tx *gorm.DB,
	season SeasonRow,
	examSetID uuid.UUID,
) (time.Time, bool, error) {
	var state examSetPublicationStateRow
	result := tx.Raw(
		findRolloverExamSetStateSQL,
		examSetID,
		season.ExamTrackID,
		examsetdomain.StatusPublished,
	).Scan(&state)
	if result.Error != nil {
		return time.Time{}, false, result.Error
	}
	if result.RowsAffected == 0 || state.ID == uuid.Nil || state.PublishedAt == nil {
		return time.Time{}, false, nil
	}
	publishedAt := *state.PublishedAt
	if publishedAt.Before(season.StartsAt) {
		publishedAt = season.StartsAt
	}
	return publishedAt, true, nil
}

const acquireExamSetTransitionLockSQL = `
	SELECT pg_advisory_xact_lock(hashtextextended(CAST(? AS text), 0))
`

const acquireSeasonProjectionLockSQL = `
	SELECT pg_advisory_xact_lock(hashtextextended(CAST(? AS text), 1))
`

const findSeasonStatusAfterProjectionLockSQL = `
	SELECT status
	FROM leaderboard_seasons
	WHERE id = ?
	FOR SHARE
`

const findExactExamSetIntervalSQL = `
	SELECT EXISTS (
		SELECT 1
		FROM leaderboard_season_exam_sets
		WHERE season_id = ? AND exam_set_id = ? AND joined_at = ?
	)
`

const findContainingExamSetIntervalSQL = `
	SELECT id, joined_at, stopped_at
	FROM leaderboard_season_exam_sets
	WHERE season_id = ?
		AND exam_set_id = ?
		AND joined_at < ?
		AND (stopped_at IS NULL OR stopped_at > ?)
	ORDER BY joined_at DESC
	LIMIT 1
	FOR UPDATE
`

const findNextExamSetIntervalSQL = `
	SELECT joined_at
	FROM leaderboard_season_exam_sets
	WHERE season_id = ? AND exam_set_id = ? AND joined_at > ?
	ORDER BY joined_at
	LIMIT 1
	FOR UPDATE
`

const closeContainingExamSetIntervalSQL = `
	UPDATE leaderboard_season_exam_sets
	SET stopped_at = ?
	WHERE id = ?
		AND joined_at < ?
		AND (stopped_at IS NULL OR stopped_at > ?)
`

const insertSeasonExamSetIntervalSQL = `
	INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at, stopped_at)
	VALUES (?, ?, ?, ?, ?)
`

const insertExamSetStopEventSQL = `
	INSERT INTO leaderboard_exam_set_stop_events (exam_set_id, stopped_at)
	VALUES (?, ?)
	ON CONFLICT (exam_set_id, stopped_at) DO NOTHING
`

const findNextExamSetStopEventSQL = `
	SELECT stopped_at
	FROM leaderboard_exam_set_stop_events
	WHERE exam_set_id = ? AND stopped_at >= ?
	ORDER BY stopped_at
	LIMIT 1
`

const stopContainingExamSetIntervalsSQL = `
	UPDATE leaderboard_season_exam_sets
	SET stopped_at = ?
	WHERE exam_set_id = ?
		AND joined_at <= ?
		AND (stopped_at IS NULL OR stopped_at > ?)
`

func (r *postgresRepository) JoinExamSet(ctx context.Context, seasonID, examSetID uuid.UUID, joinedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}
		return publishExamSetInterval(tx, seasonID, examSetID, joinedAt)
	})
}

func publishExamSetInterval(tx *gorm.DB, seasonID, examSetID uuid.UUID, joinedAt time.Time) error {
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

	var containingInterval struct {
		ID        uuid.UUID
		JoinedAt  time.Time
		StoppedAt *time.Time
	}
	if err := tx.Raw(
		findContainingExamSetIntervalSQL,
		seasonID, examSetID, joinedAt, joinedAt,
	).Scan(&containingInterval).Error; err != nil {
		return err
	}
	if containingInterval.ID != uuid.Nil {
		closed := tx.Exec(
			closeContainingExamSetIntervalSQL,
			joinedAt, containingInterval.ID, joinedAt, joinedAt,
		)
		if closed.Error != nil {
			return closed.Error
		}
		if closed.RowsAffected != 1 {
			return fmt.Errorf("close leaderboard enrollment interval: affected %d rows", closed.RowsAffected)
		}
	}

	var nextInterval struct {
		JoinedAt time.Time
	}
	if err := tx.Raw(
		findNextExamSetIntervalSQL,
		seasonID, examSetID, joinedAt,
	).Scan(&nextInterval).Error; err != nil {
		return err
	}
	var nextStop struct {
		StoppedAt time.Time
	}
	if err := tx.Raw(
		findNextExamSetStopEventSQL,
		examSetID, joinedAt,
	).Scan(&nextStop).Error; err != nil {
		return err
	}
	var stoppedAt *time.Time
	if !nextInterval.JoinedAt.IsZero() {
		boundary := nextInterval.JoinedAt
		stoppedAt = &boundary
	}
	if !nextStop.StoppedAt.IsZero() && (stoppedAt == nil || nextStop.StoppedAt.Before(*stoppedAt)) {
		boundary := nextStop.StoppedAt
		stoppedAt = &boundary
	}
	return tx.Exec(
		insertSeasonExamSetIntervalSQL,
		uuid.New(), seasonID, examSetID, joinedAt, stoppedAt,
	).Error
}

func (r *postgresRepository) StopExamSet(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}
		if err := tx.Exec(insertExamSetStopEventSQL, examSetID, stoppedAt).Error; err != nil {
			return err
		}
		return tx.Exec(
			stopContainingExamSetIntervalsSQL,
			stoppedAt, examSetID, stoppedAt, stoppedAt,
		).Error
	})
}

func (r *postgresRepository) GetEligibleSeason(ctx context.Context, examSetID uuid.UUID, submittedAt time.Time) (*SeasonRow, error) {
	return getEligibleSeason(r.db.WithContext(ctx), examSetID, submittedAt)
}

type eligibleSeasonIntervalRow struct {
	SeasonRow
	IntervalStoppedAt *time.Time
}

type examSetPublicationStateRow struct {
	ID          uuid.UUID
	ExamTrackID uuid.UUID
	Status      string
	IsActive    bool
	PublishedAt *time.Time
}

func (s examSetPublicationStateRow) currentlyPublished() bool {
	return s.Status == examsetdomain.StatusPublished && s.IsActive
}

const findExamSetPublicationStateSQL = `
	SELECT id, exam_track_id, status, is_active, published_at
	FROM exam_sets
	WHERE id = ?
`

const findExamSetPublicationStateForShareSQL = `
	SELECT id, exam_track_id, status, is_active, published_at
	FROM exam_sets
	WHERE id = ?
	FOR SHARE
`

func loadExamSetPublicationState(
	db *gorm.DB,
	examSetID uuid.UUID,
	forShare bool,
) (*examSetPublicationStateRow, error) {
	query := findExamSetPublicationStateSQL
	if forShare {
		query = findExamSetPublicationStateForShareSQL
	}
	var state examSetPublicationStateRow
	result := db.Raw(query, examSetID).Scan(&state)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 || state.ID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}
	return &state, nil
}

func validateExamSetPublishState(state *examSetPublicationStateRow, examTrackID uuid.UUID) error {
	if state.ExamTrackID != examTrackID {
		return fmt.Errorf(
			"exam set %s belongs to track %s, not %s",
			state.ID,
			state.ExamTrackID,
			examTrackID,
		)
	}
	return nil
}

const findEligibleSeasonIntervalSQL = `
	SELECT
		s.id,
		s.exam_track_id,
		s.year,
		s.month,
		s.starts_at,
		s.ends_at,
		ses.stopped_at AS interval_stopped_at
	FROM leaderboard_seasons s
	JOIN leaderboard_season_exam_sets ses ON ses.season_id = s.id
	WHERE s.status = 'active'
		AND ? >= s.starts_at
		AND ? < s.ends_at
		AND ses.exam_set_id = ?
		AND ? >= ses.joined_at
		AND (ses.stopped_at IS NULL OR ? < ses.stopped_at)
	ORDER BY s.starts_at DESC, ses.joined_at DESC
	LIMIT 1
`

const findSeasonForExamSetAtSQL = `
	SELECT s.id, s.exam_track_id, s.year, s.month, s.starts_at, s.ends_at
	FROM leaderboard_seasons s
	JOIN exam_sets es ON es.exam_track_id = s.exam_track_id
	WHERE es.id = ?
		AND s.status = 'active'
		AND ? >= s.starts_at
		AND ? < s.ends_at
	ORDER BY s.starts_at DESC
	LIMIT 1
`

func getEligibleSeason(db *gorm.DB, examSetID uuid.UUID, submittedAt time.Time) (*SeasonRow, error) {
	row, err := getEligibleSeasonInterval(db, examSetID, submittedAt)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	season := row.SeasonRow
	return &season, nil
}

func getEligibleSeasonInterval(
	db *gorm.DB,
	examSetID uuid.UUID,
	submittedAt time.Time,
) (*eligibleSeasonIntervalRow, error) {
	var row eligibleSeasonIntervalRow
	err := db.Raw(
		findEligibleSeasonIntervalSQL,
		submittedAt,
		submittedAt,
		examSetID,
		submittedAt,
		submittedAt,
	).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) ProjectBestScore(
	ctx context.Context,
	userID, examSetID, attemptID uuid.UUID,
	submittedAt time.Time,
	candidate domain.ScoreCandidate,
) (*BestScoreProjection, error) {
	projection := &BestScoreProjection{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(acquireExamSetTransitionLockSQL, examSetID).Error; err != nil {
			return err
		}

		publicationState, err := loadExamSetPublicationState(tx, examSetID, true)
		if err != nil {
			return err
		}
		season, err := resolveProjectionSeason(tx, examSetID, submittedAt, *publicationState)
		if err != nil {
			return err
		}
		if season == nil {
			return nil
		}
		projection.Season = season

		if err := tx.Exec(acquireSeasonProjectionLockSQL, season.ID).Error; err != nil {
			return err
		}
		var lockedSeason struct {
			Status string
		}
		if err := tx.Raw(findSeasonStatusAfterProjectionLockSQL, season.ID).Scan(&lockedSeason).Error; err != nil {
			return err
		}
		if lockedSeason.Status != "active" {
			projection.Season = nil
			return nil
		}
		projection.PreviousRank, err = getUserRank(tx, season.ID, userID)
		if err != nil {
			return err
		}
		projection.ScoreUpdate, err = upsertBestScore(
			tx,
			season.ID,
			userID,
			examSetID,
			attemptID,
			candidate,
		)
		if err != nil {
			return err
		}
		projection.Entry, err = rebuildEntry(tx, season.ID, userID)
		if err != nil {
			return err
		}
		projection.CurrentRank, err = getUserRank(tx, season.ID, userID)
		return err
	}, projectionTxOptions)
	if err != nil {
		return nil, err
	}
	return projection, nil
}

func resolveProjectionSeason(
	tx *gorm.DB,
	examSetID uuid.UUID,
	submittedAt time.Time,
	publicationState examSetPublicationStateRow,
) (*SeasonRow, error) {
	eligible, err := getEligibleSeasonInterval(tx, examSetID, submittedAt)
	if err != nil {
		return nil, err
	}
	if eligible != nil {
		if eligible.IntervalStoppedAt == nil && !publicationState.currentlyPublished() {
			return nil, ErrLifecycleStatePending
		}
		season := eligible.SeasonRow
		return &season, nil
	}

	var season SeasonRow
	if err := tx.Raw(
		findSeasonForExamSetAtSQL,
		examSetID,
		submittedAt,
		submittedAt,
	).Scan(&season).Error; err != nil {
		return nil, err
	}
	if season.ID == uuid.Nil {
		return nil, nil
	}

	if !publicationState.currentlyPublished() {
		if publicationState.PublishedAt == nil || submittedAt.Before(*publicationState.PublishedAt) {
			return nil, nil
		}
		var intervalCount int64
		if err := tx.Raw(`
			SELECT COUNT(*)
			FROM leaderboard_season_exam_sets
			WHERE season_id = ? AND exam_set_id = ?
		`, season.ID, examSetID).Scan(&intervalCount).Error; err != nil {
			return nil, err
		}
		if intervalCount == 0 {
			return nil, ErrLifecycleStatePending
		}
		return nil, nil
	}
	if publicationState.PublishedAt == nil {
		return nil, ErrLifecycleStatePending
	}

	joinedAt := *publicationState.PublishedAt
	if joinedAt.Before(season.StartsAt) {
		joinedAt = season.StartsAt
	}
	if submittedAt.Before(joinedAt) {
		return nil, nil
	}
	if err := publishExamSetInterval(tx, season.ID, examSetID, joinedAt); err != nil {
		return nil, fmt.Errorf("%w: self-heal publish interval: %v", ErrLifecycleStatePending, err)
	}
	eligible, err = getEligibleSeasonInterval(tx, examSetID, submittedAt)
	if err != nil {
		return nil, err
	}
	if eligible == nil {
		return nil, ErrLifecycleStatePending
	}
	resolved := eligible.SeasonRow
	return &resolved, nil
}

func (r *postgresRepository) UpsertBestScore(
	ctx context.Context,
	seasonID, userID, examSetID, attemptID uuid.UUID,
	candidate domain.ScoreCandidate,
) (*BestScoreUpdate, error) {
	var update BestScoreUpdate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		projected, err := upsertBestScore(tx, seasonID, userID, examSetID, attemptID, candidate)
		if err == nil {
			update = *projected
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &update, nil
}

func upsertBestScore(
	tx *gorm.DB,
	seasonID, userID, examSetID, attemptID uuid.UUID,
	candidate domain.ScoreCandidate,
) (*BestScoreUpdate, error) {
	var update BestScoreUpdate
	insert := tx.Exec(`
			INSERT INTO leaderboard_scores (
				season_id, user_id, exam_set_id, attempt_id,
				points, duration_seconds, achieved_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (season_id, user_id, exam_set_id) DO NOTHING
		`, seasonID, userID, examSetID, attemptID, candidate.Points, candidate.DurationSeconds, candidate.AchievedAt)
	if insert.Error != nil {
		return nil, insert.Error
	}
	if insert.RowsAffected == 1 {
		update.Current = candidate
		update.Improved = true
		return &update, nil
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
		return nil, err
	}

	previous := domain.ScoreCandidate{
		Points:          current.Points,
		DurationSeconds: current.DurationSeconds,
		AchievedAt:      current.AchievedAt,
	}
	update.Previous = &previous
	update.Current = previous
	if !domain.AttemptWins(candidate, previous) {
		return &update, nil
	}

	if err := tx.Exec(`
			UPDATE leaderboard_scores
			SET attempt_id = ?, points = ?, duration_seconds = ?, achieved_at = ?, updated_at = now()
			WHERE season_id = ? AND user_id = ? AND exam_set_id = ?
		`, attemptID, candidate.Points, candidate.DurationSeconds, candidate.AchievedAt, seasonID, userID, examSetID).Error; err != nil {
		return nil, err
	}
	update.Current = candidate
	update.Improved = true
	return &update, nil
}

func (r *postgresRepository) RebuildEntry(ctx context.Context, seasonID, userID uuid.UUID) (*EntryRow, error) {
	var entry EntryRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rebuilt, err := rebuildEntry(tx, seasonID, userID)
		if err == nil {
			entry = *rebuilt
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func rebuildEntry(tx *gorm.DB, seasonID, userID uuid.UUID) (*EntryRow, error) {
	var entry EntryRow
	var lockedAttempts []uuid.UUID
	if err := tx.Raw(`
			SELECT attempt_id
			FROM leaderboard_scores
			WHERE season_id = ? AND user_id = ?
			ORDER BY exam_set_id
			FOR UPDATE
		`, seasonID, userID).Scan(&lockedAttempts).Error; err != nil {
		return nil, err
	}
	if len(lockedAttempts) == 0 {
		if err := tx.Exec(`
				DELETE FROM leaderboard_entries WHERE season_id = ? AND user_id = ?
			`, seasonID, userID).Error; err != nil {
			return nil, err
		}
		return &entry, nil
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
		return nil, err
	}

	if err := tx.Exec(`
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
		`, seasonID, userID, entry.TotalPoints, entry.CompletedExamSets, entry.TotalDurationSeconds, entry.ScoreAchievedAt).Error; err != nil {
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
	return getUserRank(r.db.WithContext(ctx), seasonID, userID)
}

func getUserRank(db *gorm.DB, seasonID, userID uuid.UUID) (*UserRankRow, error) {
	var row UserRankRow
	err := db.Raw(monthlyRankedEntriesCTE+`
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

func (r *postgresRepository) FindMostRecentAttemptedTrack(ctx context.Context, userID uuid.UUID) (*ExamTrackContextRow, error) {
	var row ExamTrackContextRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT et.id, et.code, et.name
		FROM exam_attempts ea
		JOIN exam_sets es ON es.id = ea.exam_set_id
		JOIN exam_tracks et ON et.id = es.exam_track_id
		WHERE ea.user_id = ?
			AND ea.status IN ('submitted', 'timeout')
			AND et.is_active = true
		ORDER BY ea.submitted_at DESC NULLS LAST, ea.id
		LIMIT 1
	`, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) FindSeason(ctx context.Context, trackID uuid.UUID, year, month int) (*SeasonRow, error) {
	var row SeasonRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, exam_track_id, year, month, starts_at, ends_at, status, finalized_at
		FROM leaderboard_seasons
		WHERE exam_track_id = ? AND year = ? AND month = ?
	`, trackID, year, month).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

const monthlyPublicRankedEntriesCTE = `
	WITH visible_entries AS (
		SELECT
			e.user_id,
			u.display_name,
			u.email,
			e.total_points,
			e.completed_exam_sets,
			e.total_duration_seconds,
			e.score_achieved_at
		FROM leaderboard_entries e
		JOIN leaderboard_seasons s ON s.id = e.season_id
		JOIN users u ON u.id = e.user_id
		WHERE e.season_id = ?
			AND (s.status = 'finalized' OR u.status = 'active')
	),
	ranked AS (
		SELECT
			RANK() OVER (ORDER BY ` + monthlyRankWindowSQL + `) AS rank,
			user_id,
			display_name,
			email,
			total_points,
			completed_exam_sets,
			total_duration_seconds,
			score_achieved_at
		FROM visible_entries
	),
	ordered AS (
		SELECT * FROM ranked ORDER BY rank, user_id
	)
`

func (r *postgresRepository) CountSeasonLeaderboard(ctx context.Context, seasonID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Raw(monthlyPublicRankedEntriesCTE+`
		SELECT COUNT(*) FROM ordered
	`, seasonID).Scan(&count).Error
	return count, err
}

func (r *postgresRepository) ListSeasonLeaderboard(
	ctx context.Context,
	seasonID uuid.UUID,
	offset, limit int,
) ([]SeasonLeaderboardRow, error) {
	var rows []SeasonLeaderboardRow
	err := r.db.WithContext(ctx).Raw(monthlyPublicRankedEntriesCTE+`
		SELECT
			rank, user_id, display_name, email, total_points,
			completed_exam_sets, total_duration_seconds, score_achieved_at
		FROM ordered
		ORDER BY rank, user_id
		LIMIT ? OFFSET ?
	`, seasonID, limit, offset).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListSeasonTopThree(ctx context.Context, seasonID uuid.UUID) ([]SeasonLeaderboardRow, error) {
	var rows []SeasonLeaderboardRow
	err := r.db.WithContext(ctx).Raw(monthlyPublicRankedEntriesCTE+`
		SELECT
			rank, user_id, display_name, email, total_points,
			completed_exam_sets, total_duration_seconds, score_achieved_at
		FROM ordered
		WHERE rank <= 3
		ORDER BY rank, user_id
	`, seasonID).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListSeasonLeaderboardAroundUser(
	ctx context.Context,
	seasonID, userID uuid.UUID,
	above, below int,
) ([]SeasonLeaderboardRow, error) {
	var rows []SeasonLeaderboardRow
	err := r.db.WithContext(ctx).Raw(monthlyPublicRankedEntriesCTE+`,
	positioned AS (
		SELECT ordered.*, ROW_NUMBER() OVER (ORDER BY rank, user_id) AS row_position
		FROM ordered
	),
	current_position AS (
		SELECT row_position FROM positioned WHERE user_id = ?
	)
	SELECT
		p.rank, p.user_id, p.display_name, p.email, p.total_points,
		p.completed_exam_sets, p.total_duration_seconds, p.score_achieved_at
	FROM positioned p
	CROSS JOIN current_position c
	WHERE p.row_position BETWEEN GREATEST(1, c.row_position - ?) AND c.row_position + ?
	ORDER BY p.rank, p.user_id
	`, seasonID, userID, above, below).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) GetSeasonUserSummary(
	ctx context.Context,
	seasonID, userID uuid.UUID,
) (*SeasonUserSummaryRow, error) {
	var row SeasonUserSummaryRow
	err := r.db.WithContext(ctx).Raw(monthlyPublicRankedEntriesCTE+`
		SELECT
			rank, user_id, total_points, completed_exam_sets,
			total_duration_seconds, score_achieved_at
		FROM ordered
		WHERE user_id = ?
	`, seasonID, userID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.UserID == uuid.Nil {
		return nil, nil
	}
	return &row, nil
}

func (r *postgresRepository) ListNextOpportunities(
	ctx context.Context,
	seasonID, userID uuid.UUID,
) ([]NextOpportunityRow, error) {
	var rows []NextOpportunityRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT es.code, es.title, et.name AS exam_track_name
		FROM leaderboard_season_exam_sets ses
		JOIN leaderboard_seasons s ON s.id = ses.season_id
		JOIN exam_sets es ON es.id = ses.exam_set_id
		JOIN exam_tracks et ON et.id = s.exam_track_id
		WHERE ses.season_id = ?
			AND s.status = 'active'
			AND ses.stopped_at IS NULL
			AND es.status = ?
			AND es.is_active = true
			AND NOT EXISTS (
				SELECT 1 FROM leaderboard_scores score
				WHERE score.season_id = ses.season_id
					AND score.exam_set_id = ses.exam_set_id
					AND score.user_id = ?
			)
		ORDER BY es.code, es.title
	`, seasonID, examsetdomain.StatusPublished, userID).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListAwards(ctx context.Context, userID uuid.UUID) ([]AwardRow, error) {
	var rows []AwardRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			a.season_id,
			et.code AS track_code,
			s.year,
			s.month,
			a.rank,
			a.created_at AS awarded_at
		FROM leaderboard_awards a
		JOIN leaderboard_seasons s ON s.id = a.season_id
		JOIN exam_tracks et ON et.id = s.exam_track_id
		WHERE a.user_id = ?
		ORDER BY s.year DESC, s.month DESC, a.rank, et.code, a.id
	`, userID).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) ListDueSeasons(ctx context.Context, at time.Time) ([]SeasonRow, error) {
	var rows []SeasonRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, exam_track_id, year, month, starts_at, ends_at, status, finalized_at
		FROM leaderboard_seasons
		WHERE status = 'active' AND ends_at <= ?
		ORDER BY ends_at, id
	`, at).Scan(&rows).Error
	return rows, err
}

func (r *postgresRepository) FinalizeSeason(
	ctx context.Context,
	seasonID uuid.UUID,
	finalizedAt time.Time,
) (*FinalizationResult, error) {
	result := &FinalizationResult{SeasonID: seasonID}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(acquireSeasonProjectionLockSQL, seasonID).Error; err != nil {
			return err
		}
		var season SeasonRow
		query := tx.Raw(`
			SELECT id, exam_track_id, year, month, starts_at, ends_at, status, finalized_at
			FROM leaderboard_seasons
			WHERE id = ?
			FOR UPDATE
		`, seasonID).Scan(&season)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected == 0 || season.ID == uuid.Nil {
			return gorm.ErrRecordNotFound
		}
		if season.Status == "finalized" {
			if season.FinalizedAt != nil {
				result.FinalizedAt = *season.FinalizedAt
			}
			return nil
		}
		if finalizedAt.Before(season.EndsAt) {
			return nil
		}
		if err := tx.Exec(`
			DELETE FROM leaderboard_entries entry
			WHERE entry.season_id = ?
				AND NOT EXISTS (
					SELECT 1 FROM leaderboard_scores score
					WHERE score.season_id = entry.season_id AND score.user_id = entry.user_id
				)
		`, seasonID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO leaderboard_entries (
				season_id, user_id, total_points, completed_exam_sets,
				total_duration_seconds, score_achieved_at
			)
			SELECT
				season_id,
				user_id,
				SUM(points),
				COUNT(*)::int,
				SUM(duration_seconds)::bigint,
				MAX(achieved_at)
			FROM leaderboard_scores
			WHERE season_id = ?
			GROUP BY season_id, user_id
			ON CONFLICT (season_id, user_id) DO UPDATE SET
				total_points = EXCLUDED.total_points,
				completed_exam_sets = EXCLUDED.completed_exam_sets,
				total_duration_seconds = EXCLUDED.total_duration_seconds,
				score_achieved_at = EXCLUDED.score_achieved_at,
				updated_at = now()
		`, seasonID).Error; err != nil {
			return err
		}

		awards := tx.Exec(`
			WITH ranked AS (
				SELECT
					RANK() OVER (ORDER BY `+monthlyRankWindowSQL+`) AS rank,
					user_id
				FROM leaderboard_entries
				WHERE season_id = ?
			)
			INSERT INTO leaderboard_awards (
				id, season_id, user_id, rank, created_at, updated_at
			)
			SELECT gen_random_uuid(), ?, user_id, rank, ?, ?
			FROM ranked
			WHERE rank BETWEEN 1 AND 3
			ORDER BY rank, user_id
			ON CONFLICT (season_id, user_id, rank) DO NOTHING
		`, seasonID, seasonID, finalizedAt, finalizedAt)
		if awards.Error != nil {
			return awards.Error
		}

		updated := tx.Exec(`
			UPDATE leaderboard_seasons
			SET status = 'finalized', finalized_at = ?, updated_at = now()
			WHERE id = ? AND status = 'active'
		`, finalizedAt, seasonID)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("finalize leaderboard season: affected %d rows", updated.RowsAffected)
		}

		result.Finalized = true
		result.FinalizedAt = finalizedAt
		result.AwardCount = awards.RowsAffected
		return nil
	}, projectionTxOptions)
	if err != nil {
		return nil, err
	}
	return result, nil
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
