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
		name               string
		expiresAt          time.Time
		allowAfterDeadline bool
		wantTransition     AttemptTransition
		wantStatus         string
	}{
		{
			name:           "countdown equality is timeout",
			expiresAt:      projectionTransitionFixtureTime(),
			wantTransition: AttemptTransitionTimedOut,
			wantStatus:     domain.StatusTimeout,
		},
		{
			name:               "elapsed mode submits after nominal deadline",
			expiresAt:          projectionTransitionFixtureTime().Add(-time.Minute),
			allowAfterDeadline: true,
			wantTransition:     AttemptTransitionSubmitted,
			wantStatus:         domain.StatusSubmitted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openAttemptProjectionIntegrationDB(t)
			repo := NewPostgresRepository(db)
			attempt := insertProjectionAttemptFixture(t, db, tc.expiresAt)
			setSubmittedFixture(attempt, projectionTransitionFixtureTime())

			transition, err := repo.UpdateAttemptSubmitted(t.Context(), attempt, nil, tc.allowAfterDeadline)
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
	attempt := insertProjectionAttemptFixture(t, db, projectionTransitionFixtureTime().Add(time.Minute))
	setSubmittedFixture(attempt, projectionTransitionFixtureTime())

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var submitTransition AttemptTransition
	var submitErr, timeoutErr error
	var timeoutChanged bool
	go func() {
		defer wg.Done()
		<-start
		submitTransition, submitErr = repo.UpdateAttemptSubmitted(context.Background(), attempt, nil, false)
	}()
	go func() {
		defer wg.Done()
		<-start
		timeoutChanged, timeoutErr = repo.MarkAttemptTimeout(context.Background(), attempt.ID)
	}()
	close(start)
	wg.Wait()

	if submitErr != nil || timeoutErr != nil {
		t.Fatalf("concurrent errors submit=%v timeout=%v", submitErr, timeoutErr)
	}
	if (submitTransition == AttemptTransitionSubmitted) == timeoutChanged {
		t.Fatalf("submit transition / timeout changed = %q / %v, want exactly one winner", submitTransition, timeoutChanged)
	}
	var status string
	if err := db.Table("exam_attempts").Select("status").Where("id = ?", attempt.ID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	assertAttemptStatusAndSingleOutbox(t, db, attempt.ID, status)
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
