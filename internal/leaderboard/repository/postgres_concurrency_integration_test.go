package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardusecase "virtual-exam-api/internal/leaderboard/usecase"
)

const postgresRaceBound = 5 * time.Second

type postgresEnsureResult struct {
	season *leaderboardrepo.SeasonRow
	err    error
}

func TestPostgresInitialBootstrapUsesPersistedPublicationStateTime(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	effectivePublishedAt := window.StartsAt.Add(48 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, window.StartsAt.Add(-time.Hour), effectivePublishedAt)

	season, err := repo.EnsureSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("EnsureSeason() error = %v", err)
	}
	var joinedAt time.Time
	if err := db.Raw(`
		SELECT joined_at
		FROM leaderboard_season_exam_sets
		WHERE season_id = ? AND exam_set_id = ?
	`, season.ID, examSetID).Scan(&joinedAt).Error; err != nil {
		t.Fatalf("read bootstrap enrollment: %v", err)
	}
	if !joinedAt.Equal(effectivePublishedAt) {
		t.Errorf("bootstrap joined_at = %s, want %s", joinedAt, effectivePublishedAt)
	}
}

func TestPostgresRolloverWaitsForStopAndDoesNotBackdateEnrollment(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	previousWindow := mustBangkokWindow(t, time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC))
	currentWindow := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	previousSeasonID := uuid.New()

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, previousWindow.StartsAt, previousWindow.StartsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, previousSeasonID, trackID, previousWindow.Year, previousWindow.Month, previousWindow.StartsAt, previousWindow.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), previousSeasonID, examSetID, previousWindow.StartsAt)

	blocker := beginBlockedPublicationTransition(t, ctx, db, examSetID, "draft")
	stopResult := make(chan error, 1)
	go func() {
		stopResult <- repo.StopExamSet(ctx, examSetID, currentWindow.StartsAt)
	}()
	assertStillBlocked(t, "stop", stopResult)

	ensureResults := make(chan postgresEnsureResult, 1)
	go func() {
		season, err := repo.EnsureSeason(ctx, trackID, currentWindow)
		ensureResults <- postgresEnsureResult{season: season, err: err}
	}()

	ensureCompletedBeforeTransition := false
	var ensured postgresEnsureResult
	select {
	case ensured = <-ensureResults:
		ensureCompletedBeforeTransition = true
	case <-time.After(150 * time.Millisecond):
	}

	if err := blocker.Commit(); err != nil {
		t.Fatalf("commit blocked publication transition: %v", err)
	}
	if err := awaitError(t, "stop", stopResult); err != nil {
		t.Fatalf("StopExamSet() error = %v", err)
	}
	if !ensureCompletedBeforeTransition {
		ensured = awaitEnsureResult(t, ensureResults)
	}
	if ensured.err != nil {
		t.Fatalf("EnsureSeason() error = %v", ensured.err)
	}
	if ensureCompletedBeforeTransition {
		t.Error("EnsureSeason() completed while the lifecycle advisory lock was held")
	}

	var intervalCount int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM leaderboard_season_exam_sets ses
		JOIN leaderboard_seasons s ON s.id = ses.season_id
		WHERE s.exam_track_id = ? AND s.year = ? AND s.month = ? AND ses.exam_set_id = ?
	`, trackID, currentWindow.Year, currentWindow.Month, examSetID).Scan(&intervalCount).Error; err != nil {
		t.Fatalf("count rollover intervals: %v", err)
	}
	if intervalCount != 0 {
		t.Errorf("current-season intervals = %d, want 0 after stop won rollover", intervalCount)
	}
}

func TestPostgresProjectionWaitsForStopAndRechecksPublicationState(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	submittedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?)`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id) VALUES (?)`, attemptID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)

	blocker := beginBlockedPublicationTransition(t, ctx, db, examSetID, "draft")
	stopResult := make(chan error, 1)
	go func() {
		stopResult <- repo.StopExamSet(ctx, examSetID, submittedAt)
	}()
	assertStillBlocked(t, "stop", stopResult)

	type projectionResult struct {
		update *domain.ProjectionUpdate
		err    error
	}
	projectionResults := make(chan projectionResult, 1)
	go func() {
		update, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
			AttemptID:   attemptID,
			UserID:      userID,
			ExamSetID:   examSetID,
			ExamTrackID: trackID,
			TrackCode:   "integration-track",
			SubmittedAt: submittedAt,
			Candidate: domain.ScoreCandidate{
				Points:          91.2,
				DurationSeconds: 600,
				AchievedAt:      submittedAt,
			},
		})
		projectionResults <- projectionResult{update: update, err: err}
	}()

	projectionCompletedBeforeTransition := false
	var projected projectionResult
	select {
	case projected = <-projectionResults:
		projectionCompletedBeforeTransition = true
	case <-time.After(150 * time.Millisecond):
	}

	if err := blocker.Commit(); err != nil {
		t.Fatalf("commit blocked publication transition: %v", err)
	}
	if err := awaitError(t, "stop", stopResult); err != nil {
		t.Fatalf("StopExamSet() error = %v", err)
	}
	if !projectionCompletedBeforeTransition {
		select {
		case projected = <-projectionResults:
		case <-time.After(postgresRaceBound):
			t.Fatal("ProjectAttempt() did not finish within the bounded wait")
		}
	}
	if projected.err != nil {
		t.Fatalf("ProjectAttempt() error = %v", projected.err)
	}
	if projectionCompletedBeforeTransition {
		t.Error("ProjectAttempt() completed while the lifecycle advisory lock was held")
	}
	if projected.update == nil {
		t.Fatal("ProjectAttempt() update = nil")
	}
	if projected.update.SeasonID != "" {
		t.Errorf("ProjectAttempt() season = %q, want no eligible season after stop", projected.update.SeasonID)
	}

	for _, table := range []string{"leaderboard_scores", "leaderboard_entries"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM " + table).Scan(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s rows = %d, want 0", table, count)
		}
	}
}

func TestPostgresProjectionReturnsRankMovementAndRebuiltAggregate(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	competitorID := uuid.New()
	oldAttemptID := uuid.New()
	newAttemptID := uuid.New()
	competitorAttemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	achievedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?), (?)`, userID, competitorID)
	mustExec(t, db, `
		INSERT INTO exam_attempts (id) VALUES (?), (?), (?)
	`, oldAttemptID, newAttemptID, competitorAttemptID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_scores (
			season_id, user_id, exam_set_id, attempt_id, points, duration_seconds, achieved_at
		) VALUES
			(?, ?, ?, ?, 80, 700, ?),
			(?, ?, ?, ?, 85, 650, ?)
	`, seasonID, userID, examSetID, oldAttemptID, achievedAt,
		seasonID, competitorID, examSetID, competitorAttemptID, achievedAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_entries (
			season_id, user_id, total_points, completed_exam_sets,
			total_duration_seconds, score_achieved_at
		) VALUES
			(?, ?, 80, 1, 700, ?),
			(?, ?, 85, 1, 650, ?)
	`, seasonID, userID, achievedAt, seasonID, competitorID, achievedAt)

	update, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
		AttemptID:   newAttemptID,
		UserID:      userID,
		ExamSetID:   examSetID,
		ExamTrackID: trackID,
		TrackCode:   "integration-track",
		SubmittedAt: achievedAt.Add(time.Hour),
		Candidate: domain.ScoreCandidate{
			Points:          90,
			DurationSeconds: 600,
			AchievedAt:      achievedAt.Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("ProjectAttempt() error = %v", err)
	}
	if update.PreviousRank == nil || *update.PreviousRank != 2 {
		t.Errorf("PreviousRank = %v, want 2", update.PreviousRank)
	}
	if update.CurrentRank != 1 {
		t.Errorf("CurrentRank = %d, want 1", update.CurrentRank)
	}
	if update.BestScoreBefore != 80 || update.BestScoreAfter != 90 || update.PointsAdded != 10 {
		t.Errorf(
			"best before/after/added = %.1f/%.1f/%.1f, want 80/90/10",
			update.BestScoreBefore,
			update.BestScoreAfter,
			update.PointsAdded,
		)
	}
	if update.TotalPoints != 90 || !update.ImprovedBestScore {
		t.Errorf("total/improved = %.1f/%t, want 90/true", update.TotalPoints, update.ImprovedBestScore)
	}
}

func TestPostgresProjectionRollsBackScoreWhenEntryRebuildFails(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	submittedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?)`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id) VALUES (?)`, attemptID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)
	mustExec(t, db, `
		CREATE FUNCTION reject_leaderboard_entry() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced entry rebuild failure';
		END;
		$$ LANGUAGE plpgsql
	`)
	mustExec(t, db, `
		CREATE TRIGGER reject_leaderboard_entry
		BEFORE INSERT OR UPDATE ON leaderboard_entries
		FOR EACH ROW EXECUTE FUNCTION reject_leaderboard_entry()
	`)

	_, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
		AttemptID:   attemptID,
		UserID:      userID,
		ExamSetID:   examSetID,
		ExamTrackID: trackID,
		TrackCode:   "integration-track",
		SubmittedAt: submittedAt,
		Candidate: domain.ScoreCandidate{
			Points:          90,
			DurationSeconds: 600,
			AchievedAt:      submittedAt,
		},
	})
	if err == nil {
		t.Fatal("ProjectAttempt() error = nil, want forced aggregate rebuild failure")
	}

	for _, table := range []string{"leaderboard_scores", "leaderboard_entries"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM " + table).Scan(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s rows = %d, want 0 after transaction rollback", table, count)
		}
	}
}

func openLeaderboardIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("LEADERBOARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEADERBOARD_POSTGRES_DSN is not set")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	schemaName := "leaderboard_task3_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create integration schema: %v", err)
	}

	testDB, err := gorm.Open(
		postgres.Open(dsnWithSearchPath(dsn, schemaName)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open schema-scoped PostgreSQL connection: %v", err)
	}
	testSQLDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("access schema-scoped sql.DB: %v", err)
	}
	testSQLDB.SetMaxOpenConns(8)

	adminSQLDB, err := admin.DB()
	if err != nil {
		t.Fatalf("access admin sql.DB: %v", err)
	}
	t.Cleanup(func() {
		_ = testSQLDB.Close()
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + schemaName + " CASCADE").Error
		_ = adminSQLDB.Close()
	})

	for _, statement := range []string{
		`CREATE TABLE exam_tracks (id uuid PRIMARY KEY)`,
		`CREATE TABLE exam_sets (
			id uuid PRIMARY KEY,
			exam_track_id uuid NOT NULL REFERENCES exam_tracks(id),
			status varchar(50) NOT NULL,
			is_active boolean NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE users (id uuid PRIMARY KEY)`,
		`CREATE TABLE exam_attempts (id uuid PRIMARY KEY)`,
	} {
		mustExec(t, testDB, statement)
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration test file")
	}
	migrationPath := filepath.Join(filepath.Dir(currentFile), "../../../migrations/000023_monthly_leaderboards.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration 000023: %v", err)
	}
	if _, err := testSQLDB.ExecContext(t.Context(), string(migration)); err != nil {
		t.Fatalf("apply migration 000023: %v", err)
	}

	return testDB
}

func dsnWithSearchPath(dsn, schemaName string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schemaName)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return fmt.Sprintf("%s search_path=%s", dsn, schemaName)
}

func beginBlockedPublicationTransition(
	t *testing.T,
	ctx context.Context,
	db *gorm.DB,
	examSetID uuid.UUID,
	status string,
) *sql.Tx {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sql.DB: %v", err)
	}
	connection, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve blocker connection: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin blocker transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended(CAST($1 AS text), 0))`,
		examSetID,
	); err != nil {
		t.Fatalf("acquire blocker advisory lock: %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE exam_sets SET status = $1, updated_at = now() WHERE id = $2`,
		status,
		examSetID,
	); err != nil {
		t.Fatalf("stage publication state transition: %v", err)
	}
	return tx
}

func assertStillBlocked(t *testing.T, operation string, results <-chan error) {
	t.Helper()
	select {
	case err := <-results:
		t.Fatalf("%s completed before lock release: %v", operation, err)
	case <-time.After(75 * time.Millisecond):
	}
}

func awaitError(t *testing.T, operation string, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(postgresRaceBound):
		t.Fatalf("%s did not finish within the bounded wait", operation)
		return nil
	}
}

func awaitEnsureResult(
	t *testing.T,
	results <-chan postgresEnsureResult,
) postgresEnsureResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(postgresRaceBound):
		t.Fatal("EnsureSeason() did not finish within the bounded wait")
		return postgresEnsureResult{}
	}
}

func mustBangkokWindow(t *testing.T, instant time.Time) domain.SeasonWindow {
	t.Helper()
	window, err := domain.BangkokSeasonWindow(instant)
	if err != nil {
		t.Fatalf("BangkokSeasonWindow() error = %v", err)
	}
	return window
}

func mustExec(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("execute integration fixture SQL: %v", err)
	}
}
