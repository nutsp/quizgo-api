package repository_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardusecase "virtual-exam-api/internal/leaderboard/usecase"
)

func TestPostgresReconcileSeasonMatchesNormalProjectionAndIsIdempotent(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	userID := uuid.New()
	closedSetID := uuid.New()
	openSetID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	joinedAt := window.StartsAt.Add(5 * 24 * time.Hour)
	stoppedAt := window.StartsAt.Add(20 * 24 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id, code) VALUES (?, 'reconcile-track')`, trackID)
	mustExec(t, db, `INSERT INTO users (id, email) VALUES (?, 'reconcile@example.com')`, userID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, status, is_active, published_at)
		VALUES
			(?, ?, 'closed-set', 'draft', false, ?),
			(?, ?, 'open-set', 'published', true, ?)
	`, closedSetID, trackID, joinedAt, openSetID, trackID, window.StartsAt)
	season, err := repo.EnsureSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("EnsureSeason() error = %v", err)
	}
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at, stopped_at)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.New(), season.ID, closedSetID, joinedAt, stoppedAt)

	attempts := []reconcileAttemptFixture{
		{id: uuid.New(), examSetID: closedSetID, status: "submitted", submittedAt: joinedAt.Add(-time.Second), points: 99, duration: 100},
		{id: uuid.New(), examSetID: closedSetID, status: "submitted", submittedAt: joinedAt, points: 80.04, duration: 600},
		{id: uuid.New(), examSetID: closedSetID, status: "submitted", submittedAt: joinedAt.Add(2 * time.Hour), points: 80.06, duration: 700},
		{id: uuid.New(), examSetID: closedSetID, status: "submitted", submittedAt: joinedAt.Add(4 * time.Hour), points: 80.06, duration: 650},
		{id: uuid.New(), examSetID: closedSetID, status: "submitted", submittedAt: joinedAt.Add(3 * time.Hour), points: 80.06, duration: 650},
		{id: uuid.New(), examSetID: closedSetID, status: "submitted", submittedAt: stoppedAt, points: 100, duration: 50},
		{id: uuid.New(), examSetID: openSetID, status: "timeout", submittedAt: joinedAt.Add(10 * time.Hour), points: 0, duration: 900},
	}
	for _, attempt := range attempts {
		insertReconcileAttempt(t, db, trackID, userID, attempt)
		if err := repo.RecordProjectionFailure(ctx, attempt.id, context.DeadlineExceeded); err != nil {
			t.Fatalf("RecordProjectionFailure() error = %v", err)
		}
		input := domain.ProjectionInput{
			AttemptID: attempt.id, UserID: userID, ExamSetID: attempt.examSetID,
			ExamTrackID: trackID, TrackCode: "reconcile-track", SubmittedAt: attempt.submittedAt,
			Candidate: domain.ScoreCandidate{
				Points: attempt.points, DurationSeconds: attempt.duration, AchievedAt: attempt.submittedAt,
			},
		}
		if _, err := projector.ProjectAttempt(ctx, input); err != nil {
			t.Fatalf("ProjectAttempt(%s) error = %v", attempt.id, err)
		}
		if attempt.id == attempts[3].id {
			if _, err := projector.ProjectAttempt(ctx, input); err != nil {
				t.Fatalf("ProjectAttempt() duplicate retry error = %v", err)
			}
		}
	}

	wantScores := readReconcileScores(t, db, season.ID)
	wantEntries := readReconcileEntries(t, db, season.ID)
	if len(wantScores) != 2 || len(wantEntries) != 1 {
		t.Fatalf("normal projection scores/entries = %d/%d, want 2/1", len(wantScores), len(wantEntries))
	}

	mustExec(t, db, `DELETE FROM leaderboard_entries WHERE season_id = ?`, season.ID)
	mustExec(t, db, `DELETE FROM leaderboard_scores WHERE season_id = ?`, season.ID)
	mustExec(t, db, `
		INSERT INTO leaderboard_scores (
			season_id, user_id, exam_set_id, attempt_id, points, duration_seconds, achieved_at
		) VALUES (?, ?, ?, ?, 99, 100, ?)
	`, season.ID, userID, closedSetID, attempts[0].id, attempts[0].submittedAt)

	result, err := repo.ReconcileSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("ReconcileSeason() error = %v", err)
	}
	if result.SeasonID != season.ID || result.ScoreCount != 2 || result.EntryCount != 1 {
		t.Errorf("ReconcileSeason() = %+v, want season %s with 2 scores and 1 entry", result, season.ID)
	}
	gotScores := readReconcileScores(t, db, season.ID)
	gotEntries := readReconcileEntries(t, db, season.ID)
	if !reflect.DeepEqual(gotScores, wantScores) {
		t.Errorf("reconciled scores = %+v, want normal projection %+v", gotScores, wantScores)
	}
	if !reflect.DeepEqual(gotEntries, wantEntries) {
		t.Errorf("reconciled entries = %+v, want normal projection %+v", gotEntries, wantEntries)
	}
	assertReconcileFailureResolution(t, db, attempts, joinedAt, stoppedAt)
	resolvedBeforeRetry := readReconcileFailureTimes(t, db)

	time.Sleep(2 * time.Millisecond)
	retry, err := repo.ReconcileSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("ReconcileSeason() retry error = %v", err)
	}
	if *retry != *result {
		t.Errorf("ReconcileSeason() retry = %+v, want %+v", retry, result)
	}
	if scores := readReconcileScores(t, db, season.ID); !reflect.DeepEqual(scores, gotScores) {
		t.Errorf("retry scores = %+v, want %+v", scores, gotScores)
	}
	if entries := readReconcileEntries(t, db, season.ID); !reflect.DeepEqual(entries, gotEntries) {
		t.Errorf("retry entries = %+v, want %+v", entries, gotEntries)
	}
	if resolved := readReconcileFailureTimes(t, db); !reflect.DeepEqual(resolved, resolvedBeforeRetry) {
		t.Errorf("retry failure resolution times = %+v, want unchanged %+v", resolved, resolvedBeforeRetry)
	}
}

func TestPostgresReconcileSeasonPreservesTieAndDisabledUserPolicy(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	disabledUserID := uuid.MustParse("61000000-0000-0000-0000-000000000001")
	firstActiveUserID := uuid.MustParse("62000000-0000-0000-0000-000000000002")
	secondActiveUserID := uuid.MustParse("63000000-0000-0000-0000-000000000003")
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	achievedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id, code) VALUES (?, 'policy-track')`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, status, is_active, published_at)
		VALUES (?, ?, 'policy-set', 'published', true, ?)
	`, examSetID, trackID, window.StartsAt)
	mustExec(t, db, `
		INSERT INTO users (id, email, status)
		VALUES
			(?, 'disabled@example.com', 'disabled'),
			(?, 'first@example.com', 'active'),
			(?, 'second@example.com', 'active')
	`, disabledUserID, firstActiveUserID, secondActiveUserID)

	for _, userID := range []uuid.UUID{disabledUserID, firstActiveUserID, secondActiveUserID} {
		insertReconcileAttempt(t, db, trackID, userID, reconcileAttemptFixture{
			id: uuid.New(), examSetID: examSetID, status: "submitted",
			submittedAt: achievedAt, points: 75, duration: 300,
		})
	}

	result, err := repo.ReconcileSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("ReconcileSeason() error = %v", err)
	}
	activeRows, err := repo.ListSeasonLeaderboard(ctx, result.SeasonID, 0, 10)
	if err != nil {
		t.Fatalf("ListSeasonLeaderboard(active) error = %v", err)
	}
	assertSeasonRankRows(t, activeRows, []uuid.UUID{firstActiveUserID, secondActiveUserID}, 1)

	finalized, err := repo.FinalizeSeason(ctx, result.SeasonID, window.EndsAt)
	if err != nil {
		t.Fatalf("FinalizeSeason() error = %v", err)
	}
	if !finalized.Finalized {
		t.Fatal("FinalizeSeason() did not finalize the season")
	}
	finalRows, err := repo.ListSeasonLeaderboard(ctx, result.SeasonID, 0, 10)
	if err != nil {
		t.Fatalf("ListSeasonLeaderboard(finalized) error = %v", err)
	}
	assertSeasonRankRows(t, finalRows, []uuid.UUID{disabledUserID, firstActiveUserID, secondActiveUserID}, 1)
}

func TestPostgresReconcileSeasonSerializesWithProjector(t *testing.T) {
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
	submittedAt := window.StartsAt.Add(12 * time.Hour)
	mustExec(t, db, `INSERT INTO exam_tracks (id, code) VALUES (?, 'projector-race')`, trackID)
	mustExec(t, db, `INSERT INTO users (id, email) VALUES (?, 'projector-race@example.com')`, userID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, status, is_active, published_at)
		VALUES (?, ?, 'projector-race-set', 'published', true, ?)
	`, examSetID, trackID, window.StartsAt)
	season, err := repo.EnsureSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("EnsureSeason() error = %v", err)
	}
	insertReconcileAttempt(t, db, trackID, userID, reconcileAttemptFixture{
		id: attemptID, examSetID: examSetID, status: "submitted",
		submittedAt: submittedAt, points: 90, duration: 600,
	})

	gate := acquireSeasonProjectionGate(t, db, season.ID)
	reconcileResult := make(chan error, 1)
	go func() {
		_, err := repo.ReconcileSeason(ctx, trackID, window)
		reconcileResult <- err
	}()
	waitForPostgresLockWait(t, db, "pg_advisory_xact_lock", "hashtextextended")

	projectResult := make(chan error, 1)
	go func() {
		_, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
			AttemptID: attemptID, UserID: userID, ExamSetID: examSetID,
			ExamTrackID: trackID, TrackCode: "projector-race", SubmittedAt: submittedAt,
			Candidate: domain.ScoreCandidate{Points: 90, DurationSeconds: 600, AchievedAt: submittedAt},
		})
		projectResult <- err
	}()
	waitForPostgresLockWaitCount(t, db, 2, "pg_advisory_xact_lock", "hashtextextended")
	if err := gate.Rollback().Error; err != nil {
		t.Fatalf("release season projection gate: %v", err)
	}
	if err := <-reconcileResult; err != nil {
		t.Fatalf("concurrent ReconcileSeason() error = %v", err)
	}
	if err := <-projectResult; err != nil {
		t.Fatalf("concurrent ProjectAttempt() error = %v", err)
	}
	if scores, entries := readReconcileScores(t, db, season.ID), readReconcileEntries(t, db, season.ID); len(scores) != 1 || len(entries) != 1 || scores[0].AttemptID != attemptID || entries[0].TotalPoints != 90 {
		t.Fatalf("concurrent projection state scores=%+v entries=%+v", scores, entries)
	}
}

func TestPostgresReconcileSeasonSerializesBeforeFinalizerAndRejectsFinalizedRetry(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	submittedAt := window.StartsAt.Add(12 * time.Hour)
	mustExec(t, db, `INSERT INTO exam_tracks (id, code) VALUES (?, 'finalizer-race')`, trackID)
	mustExec(t, db, `INSERT INTO users (id, email) VALUES (?, 'finalizer-race@example.com')`, userID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, status, is_active, published_at)
		VALUES (?, ?, 'finalizer-race-set', 'published', true, ?)
	`, examSetID, trackID, window.StartsAt)
	season, err := repo.EnsureSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("EnsureSeason() error = %v", err)
	}
	insertReconcileAttempt(t, db, trackID, userID, reconcileAttemptFixture{
		id: uuid.New(), examSetID: examSetID, status: "submitted",
		submittedAt: submittedAt, points: 88, duration: 500,
	})

	gate := acquireSeasonProjectionGate(t, db, season.ID)
	reconcileResult := make(chan error, 1)
	go func() {
		_, err := repo.ReconcileSeason(ctx, trackID, window)
		reconcileResult <- err
	}()
	waitForPostgresLockWait(t, db, "pg_advisory_xact_lock", "hashtextextended")

	finalizeResult := make(chan error, 1)
	go func() {
		_, err := repo.FinalizeSeason(ctx, season.ID, window.EndsAt)
		finalizeResult <- err
	}()
	waitForPostgresLockWaitCount(t, db, 2, "pg_advisory_xact_lock", "hashtextextended")
	if err := gate.Rollback().Error; err != nil {
		t.Fatalf("release season finalization gate: %v", err)
	}
	if err := <-reconcileResult; err != nil {
		t.Fatalf("ReconcileSeason() before finalization error = %v", err)
	}
	if err := <-finalizeResult; err != nil {
		t.Fatalf("FinalizeSeason() after reconciliation error = %v", err)
	}
	if scores := readReconcileScores(t, db, season.ID); len(scores) != 1 || scores[0].Points != 88 {
		t.Fatalf("finalized scores = %+v, want reconciled source score", scores)
	}
	if _, err := repo.ReconcileSeason(ctx, trackID, window); !errors.Is(err, leaderboardrepo.ErrSeasonFinalized) {
		t.Fatalf("ReconcileSeason() finalized retry error = %v, want ErrSeasonFinalized", err)
	}
}

func TestPostgresListActiveExamTracksUsesDeterministicCodeOrder(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	firstID := uuid.MustParse("81000000-0000-0000-0000-000000000001")
	secondID := uuid.MustParse("82000000-0000-0000-0000-000000000002")

	mustExec(t, db, `
		INSERT INTO exam_tracks (id, code, is_active)
		VALUES
			(?, 'z-track', true),
			(?, 'a-track', true),
			(?, 'hidden-track', false)
	`, secondID, firstID, uuid.New())
	rows, err := repo.ListActiveExamTracks(t.Context())
	if err != nil {
		t.Fatalf("ListActiveExamTracks() error = %v", err)
	}
	if len(rows) != 2 || rows[0].ID != firstID || rows[1].ID != secondID {
		t.Fatalf("ListActiveExamTracks() = %+v, want a-track then z-track", rows)
	}
}

type reconcileAttemptFixture struct {
	id          uuid.UUID
	examSetID   uuid.UUID
	status      string
	submittedAt time.Time
	points      float64
	duration    int
}

type reconcileScoreSnapshot struct {
	UserID          uuid.UUID
	ExamSetID       uuid.UUID
	AttemptID       uuid.UUID
	Points          float64
	DurationSeconds int
	AchievedAt      time.Time
}

type reconcileEntrySnapshot struct {
	UserID               uuid.UUID
	TotalPoints          float64
	CompletedExamSets    int
	TotalDurationSeconds int64
	ScoreAchievedAt      time.Time
}

func insertReconcileAttempt(t *testing.T, db *gorm.DB, trackID, userID uuid.UUID, attempt reconcileAttemptFixture) {
	t.Helper()
	mustExec(t, db, `
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status, started_at,
			submitted_at, duration_seconds, score_percent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, attempt.id, userID, trackID, attempt.examSetID, attempt.status,
		attempt.submittedAt.Add(-time.Hour), attempt.submittedAt, attempt.duration, attempt.points)
}

func readReconcileScores(t *testing.T, db *gorm.DB, seasonID uuid.UUID) []reconcileScoreSnapshot {
	t.Helper()
	var rows []reconcileScoreSnapshot
	if err := db.Raw(`
		SELECT user_id, exam_set_id, attempt_id, points, duration_seconds, achieved_at
		FROM leaderboard_scores
		WHERE season_id = ?
		ORDER BY user_id, exam_set_id
	`, seasonID).Scan(&rows).Error; err != nil {
		t.Fatalf("read reconciliation scores: %v", err)
	}
	return rows
}

func readReconcileEntries(t *testing.T, db *gorm.DB, seasonID uuid.UUID) []reconcileEntrySnapshot {
	t.Helper()
	var rows []reconcileEntrySnapshot
	if err := db.Raw(`
		SELECT user_id, total_points, completed_exam_sets, total_duration_seconds, score_achieved_at
		FROM leaderboard_entries
		WHERE season_id = ?
		ORDER BY user_id
	`, seasonID).Scan(&rows).Error; err != nil {
		t.Fatalf("read reconciliation entries: %v", err)
	}
	return rows
}

func assertReconcileFailureResolution(
	t *testing.T,
	db *gorm.DB,
	attempts []reconcileAttemptFixture,
	joinedAt, stoppedAt time.Time,
) {
	t.Helper()
	type failureRow struct {
		AttemptID  uuid.UUID
		ResolvedAt *time.Time
	}
	var rows []failureRow
	if err := db.Raw(`
		SELECT attempt_id, resolved_at
		FROM leaderboard_projection_failures
		ORDER BY attempt_id
	`).Scan(&rows).Error; err != nil {
		t.Fatalf("read projection failures: %v", err)
	}
	resolved := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		resolved[row.AttemptID] = row.ResolvedAt != nil
	}
	for _, attempt := range attempts {
		wantResolved := attempt.examSetID != attempts[0].examSetID ||
			(!attempt.submittedAt.Before(joinedAt) && attempt.submittedAt.Before(stoppedAt))
		if resolved[attempt.id] != wantResolved {
			t.Errorf("attempt %s resolved = %t, want %t", attempt.id, resolved[attempt.id], wantResolved)
		}
	}
}

func readReconcileFailureTimes(t *testing.T, db *gorm.DB) map[uuid.UUID]*time.Time {
	t.Helper()
	type failureRow struct {
		AttemptID  uuid.UUID
		ResolvedAt *time.Time
	}
	var rows []failureRow
	if err := db.Raw(`
		SELECT attempt_id, resolved_at
		FROM leaderboard_projection_failures
		ORDER BY attempt_id
	`).Scan(&rows).Error; err != nil {
		t.Fatalf("read projection failure times: %v", err)
	}
	resolved := make(map[uuid.UUID]*time.Time, len(rows))
	for _, row := range rows {
		resolved[row.AttemptID] = row.ResolvedAt
	}
	return resolved
}

func assertSeasonRankRows(
	t *testing.T,
	rows []leaderboardrepo.SeasonLeaderboardRow,
	wantUserIDs []uuid.UUID,
	wantRank int,
) {
	t.Helper()
	if len(rows) != len(wantUserIDs) {
		t.Fatalf("leaderboard rows = %+v, want users %v", rows, wantUserIDs)
	}
	for i, row := range rows {
		if row.UserID != wantUserIDs[i] || row.Rank != wantRank {
			t.Errorf("leaderboard row %d = user %s rank %d, want user %s rank %d",
				i, row.UserID, row.Rank, wantUserIDs[i], wantRank)
		}
	}
}

func acquireSeasonProjectionGate(t *testing.T, db *gorm.DB, seasonID uuid.UUID) *gorm.DB {
	t.Helper()
	gate := db.WithContext(t.Context()).Begin()
	if gate.Error != nil {
		t.Fatalf("begin season projection gate: %v", gate.Error)
	}
	t.Cleanup(func() { _ = gate.Rollback().Error })
	if err := gate.Exec(`
		SELECT pg_advisory_xact_lock(hashtextextended(CAST(? AS text), 1))
	`, seasonID).Error; err != nil {
		t.Fatalf("acquire season projection gate: %v", err)
	}
	return gate
}
