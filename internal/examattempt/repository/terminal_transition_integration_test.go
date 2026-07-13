package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/examattempt/domain"
)

func TestPostgresTerminalTransitionIsAtomicAndEnqueuesProjection(t *testing.T) {
	tests := []struct {
		name             string
		withTimingColumn bool
		timingMode       string
		expiresAt        time.Time
		wantTransition   AttemptTransition
		wantStatus       string
	}{
		{
			name:           "schema without timing mode safely falls back to countdown at equality",
			expiresAt:      projectionTransitionFixtureTime(),
			wantTransition: AttemptTransitionTimedOut,
			wantStatus:     domain.StatusTimeout,
		},
		{
			name:             "persisted elapsed mode submits after nominal deadline",
			withTimingColumn: true,
			timingMode:       "elapsed",
			expiresAt:        projectionTransitionFixtureTime().Add(-time.Minute),
			wantTransition:   AttemptTransitionSubmitted,
			wantStatus:       domain.StatusSubmitted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openAttemptProjectionIntegrationDB(t)
			if tc.withTimingColumn {
				if err := db.Exec(`ALTER TABLE exam_attempts ADD COLUMN timing_mode text NOT NULL DEFAULT 'countdown'`).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := NewPostgresRepository(db)
			attempt := insertProjectionAttemptFixture(t, db, tc.expiresAt)
			if tc.withTimingColumn {
				if err := db.Exec(`UPDATE exam_attempts SET timing_mode = ? WHERE id = ?`, tc.timingMode, attempt.ID).Error; err != nil {
					t.Fatal(err)
				}
			}
			setSubmittedFixture(attempt, projectionTransitionFixtureTime())

			transition, err := repo.UpdateAttemptSubmitted(t.Context(), attempt, nil, false)
			if err != nil {
				t.Fatalf("UpdateAttemptSubmitted() error = %v", err)
			}
			if transition != tc.wantTransition {
				t.Fatalf("transition = %q, want %q", transition, tc.wantTransition)
			}
			assertAttemptStatusAndSingleOutbox(t, db, attempt.ID, tc.wantStatus)
		})
	}
}

func TestPostgresSubmittedTransitionDoesNotOverwriteTimeout(t *testing.T) {
	db := openAttemptProjectionIntegrationDB(t)
	repo := NewPostgresRepository(db)
	attempt := insertProjectionAttemptFixture(t, db, projectionTransitionFixtureTime().Add(time.Minute))
	if err := db.Exec(`UPDATE exam_attempts SET expires_at = CURRENT_TIMESTAMP - interval '1 second' WHERE id = ?`, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.MarkAttemptTimeout(t.Context(), attempt.ID); err != nil || !changed {
		t.Fatalf("MarkAttemptTimeout() = %t, %v", changed, err)
	}
	setSubmittedFixture(attempt, projectionTransitionFixtureTime())

	transition, err := repo.UpdateAttemptSubmitted(t.Context(), attempt, nil, false)
	if err != nil {
		t.Fatalf("UpdateAttemptSubmitted() error = %v", err)
	}
	if transition != AttemptTransitionUnchanged {
		t.Fatalf("transition = %q, want unchanged", transition)
	}
	assertAttemptStatusAndSingleOutbox(t, db, attempt.ID, domain.StatusTimeout)
}

func TestPostgresConcurrentSubmitAndTimeoutProduceOneTerminalEvent(t *testing.T) {
	db := openAttemptProjectionIntegrationDB(t)
	repo := NewPostgresRepository(db)
	attempt := insertProjectionAttemptFixture(t, db, time.Now().UTC().Add(-time.Minute))
	if err := db.Exec(`
		UPDATE exam_attempts
		SET started_at = CURRENT_TIMESTAMP - interval '10 minutes',
			expires_at = CURRENT_TIMESTAMP - interval '1 second'
		WHERE id = ?
	`, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	setSubmittedFixture(attempt, projectionTransitionFixtureTime())
	blocker := db.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	if err := blocker.Exec(`SELECT id FROM exam_attempts WHERE id = ? FOR UPDATE`, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()

	start := make(chan struct{})
	entered := make(chan struct{}, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	var submitTransition AttemptTransition
	var submitErr, timeoutErr error
	var timeoutChanged bool
	go func() {
		defer wg.Done()
		<-start
		entered <- struct{}{}
		submitTransition, submitErr = repo.UpdateAttemptSubmitted(context.Background(), attempt, nil, false)
	}()
	go func() {
		defer wg.Done()
		<-start
		entered <- struct{}{}
		timeoutChanged, timeoutErr = repo.MarkAttemptTimeout(context.Background(), attempt.ID)
	}()
	close(start)
	<-entered
	<-entered
	waitForAttemptLockWaiters(t, db, 2)
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	if submitErr != nil || timeoutErr != nil {
		t.Fatalf("concurrent errors submit=%v timeout=%v", submitErr, timeoutErr)
	}
	validWinner := (submitTransition == AttemptTransitionTimedOut && !timeoutChanged) ||
		(submitTransition == AttemptTransitionUnchanged && timeoutChanged)
	if !validWinner {
		t.Fatalf("submit transition / timeout changed = %q / %v, want one timeout owner", submitTransition, timeoutChanged)
	}
	var status string
	if err := db.Table("exam_attempts").Select("status").Where("id = ?", attempt.ID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	assertAttemptStatusAndSingleOutbox(t, db, attempt.ID, status)
}

func waitForAttemptLockWaiters(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int64
		if err := db.Raw(`
			SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database()
			  AND wait_event_type = 'Lock'
			  AND query ILIKE '%exam_attempts%'
		`).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("did not observe %d concurrent exam_attempt lock waiters", want)
}

func projectionTransitionFixtureTime() time.Time {
	return time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
}

func openAttemptProjectionIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openAttemptTimeoutIntegrationDB(t)
	statements := []string{
		`CREATE TABLE exam_answers (
			id uuid PRIMARY KEY,
			attempt_id uuid NOT NULL,
			question_id uuid NOT NULL,
			is_correct boolean,
			updated_at timestamptz NOT NULL
		)`,
		`ALTER TABLE leaderboard_attempt_projection_outbox
			ADD COLUMN delivered_at timestamptz,
			ADD COLUMN claim_token uuid,
			ADD COLUMN claimed_at timestamptz,
			ADD COLUMN delivery_attempts int NOT NULL DEFAULT 0,
			ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
			ADD COLUMN last_error text,
			ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
			ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now()`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("projection fixture schema: %v", err)
		}
	}
	return db
}

func insertProjectionAttemptFixture(t *testing.T, db *gorm.DB, expiresAt time.Time) *domain.ExamAttempt {
	t.Helper()
	attempt := &domain.ExamAttempt{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		ExamTrackID: uuid.New(),
		ExamSetID:   uuid.New(),
		Status:      domain.StatusInProgress,
		StartedAt:   projectionTransitionFixtureTime().Add(-10 * time.Minute),
		ExpiresAt:   expiresAt,
	}
	if err := db.Exec(`INSERT INTO exam_tracks (id, code) VALUES (?, 'police')`, attempt.ExamTrackID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status, started_at,
			expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, attempt.ID, attempt.UserID, attempt.ExamTrackID, attempt.ExamSetID,
		attempt.Status, attempt.StartedAt, attempt.ExpiresAt, attempt.StartedAt, attempt.StartedAt).Error; err != nil {
		t.Fatal(err)
	}
	return attempt
}

func setSubmittedFixture(attempt *domain.ExamAttempt, at time.Time) {
	duration := 600
	attempt.Status = domain.StatusSubmitted
	attempt.SubmittedAt = &at
	attempt.DurationSeconds = &duration
	attempt.Score = 8
	attempt.TotalScore = 10
	attempt.ScorePercent = 80
	attempt.CorrectCount = 8
	attempt.WrongCount = 2
}

func assertAttemptStatusAndSingleOutbox(t *testing.T, db *gorm.DB, attemptID uuid.UUID, wantStatus string) {
	t.Helper()
	var status string
	if err := db.Table("exam_attempts").Select("status").Where("id = ?", attemptID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("status = %q, want %q", status, wantStatus)
	}
	var count int64
	if err := db.Table("leaderboard_attempt_projection_outbox").Where("attempt_id = ?", attemptID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("outbox rows = %d, want 1", count)
	}
}
