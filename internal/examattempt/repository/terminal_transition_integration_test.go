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

func TestPostgresSubmitFallbackUsesOnlyDatabaseDeadline(t *testing.T) {
	t.Run("application clock ahead cannot timeout before database deadline", func(t *testing.T) {
		db := openAttemptProjectionIntegrationDB(t)
		repo := NewPostgresRepository(db)
		attempt := insertProjectionAttemptFixture(t, db, time.Now().UTC().Add(time.Hour))
		if err := db.Exec(`
			UPDATE exam_attempts
			SET started_at = CURRENT_TIMESTAMP - interval '10 minutes',
				expires_at = CURRENT_TIMESTAMP + interval '1 hour'
			WHERE id = ?
		`, attempt.ID).Error; err != nil {
			t.Fatal(err)
		}
		setSubmittedFixture(attempt, time.Now().UTC().Add(2*time.Hour))

		transition, err := repo.UpdateAttemptSubmitted(t.Context(), attempt, nil, false)
		if err != nil {
			t.Fatalf("UpdateAttemptSubmitted() error = %v", err)
		}
		if transition != AttemptTransitionUnchanged {
			t.Fatalf("transition = %q, want unchanged", transition)
		}
		assertAttemptStatusAndOutboxCount(t, db, attempt.ID, domain.StatusInProgress, 0)
	})

	for _, tc := range []struct {
		name            string
		expiresOffset   int
		wantDurationSec int
	}{
		{name: "database deadline equality", expiresOffset: 0, wantDurationSec: 1800},
		{name: "database deadline already passed", expiresOffset: -1, wantDurationSec: 1799},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openAttemptProjectionIntegrationDB(t)
			tx := db.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()

			attempt := &domain.ExamAttempt{
				ID:          uuid.New(),
				UserID:      uuid.New(),
				ExamTrackID: uuid.New(),
				ExamSetID:   uuid.New(),
				Status:      domain.StatusInProgress,
			}
			if err := tx.Exec(`INSERT INTO exam_tracks (id, code) VALUES (?, 'police')`, attempt.ExamTrackID).Error; err != nil {
				t.Fatal(err)
			}
			if err := tx.Exec(`
				INSERT INTO exam_attempts (
					id, user_id, exam_track_id, exam_set_id, status, started_at,
					expires_at, created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP - interval '30 minutes',
					CURRENT_TIMESTAMP + (? * interval '1 second'),
					CURRENT_TIMESTAMP - interval '30 minutes', CURRENT_TIMESTAMP - interval '30 minutes')
			`, attempt.ID, attempt.UserID, attempt.ExamTrackID, attempt.ExamSetID,
				attempt.Status, tc.expiresOffset).Error; err != nil {
				t.Fatal(err)
			}
			setSubmittedFixture(attempt, time.Now().UTC().Add(2*time.Hour))

			repo := NewPostgresRepository(tx)
			transition, err := repo.UpdateAttemptSubmitted(t.Context(), attempt, nil, false)
			if err != nil {
				t.Fatalf("UpdateAttemptSubmitted() error = %v", err)
			}
			if transition != AttemptTransitionTimedOut {
				t.Fatalf("transition = %q, want timed_out", transition)
			}

			var payload struct {
				Status          string `gorm:"column:status"`
				AttemptDuration int    `gorm:"column:attempt_duration"`
				OutboxDuration  int    `gorm:"column:outbox_duration"`
			}
			if err := tx.Raw(`
				SELECT ea.status,
					ea.duration_seconds AS attempt_duration,
					o.duration_seconds AS outbox_duration
				FROM exam_attempts ea
				JOIN leaderboard_attempt_projection_outbox o ON o.attempt_id = ea.id
				WHERE ea.id = ?
			`, attempt.ID).Scan(&payload).Error; err != nil {
				t.Fatal(err)
			}
			if payload.Status != domain.StatusTimeout ||
				payload.AttemptDuration != tc.wantDurationSec ||
				payload.OutboxDuration != tc.wantDurationSec {
				t.Fatalf("fallback state/durations = %s/%d/%d, want %s/%d/%d",
					payload.Status, payload.AttemptDuration, payload.OutboxDuration,
					domain.StatusTimeout, tc.wantDurationSec, tc.wantDurationSec)
			}
			if err := tx.Commit().Error; err != nil {
				t.Fatal(err)
			}
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
	assertAttemptStatusAndOutboxCount(t, db, attemptID, wantStatus, 1)
}

func assertAttemptStatusAndOutboxCount(t *testing.T, db *gorm.DB, attemptID uuid.UUID, wantStatus string, wantOutbox int64) {
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
	if count != wantOutbox {
		t.Fatalf("outbox rows = %d, want %d", count, wantOutbox)
	}
}
