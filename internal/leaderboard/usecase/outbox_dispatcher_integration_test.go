package usecase

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	"virtual-exam-api/internal/leaderboard/domain"
)

type integrationOutboxProjector struct {
	mu                 sync.Mutex
	db                 *gorm.DB
	projectCalls       int
	stopCalls          int
	stopSawPersistence bool
}

func (p *integrationOutboxProjector) ProjectAttempt(_ context.Context, _ domain.ProjectionInput) (*domain.ProjectionUpdate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.projectCalls++
	return &domain.ProjectionUpdate{CurrentRank: 1}, nil
}

func (p *integrationOutboxProjector) OnExamSetStopped(_ context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopCalls++
	var active bool
	var pending int64
	stateErr := p.db.Table("exam_sets").Select("is_active").Where("id = ?", examSetID).Scan(&active).Error
	eventErr := p.db.Table("exam_set_lifecycle_stop_events").
		Where("exam_set_id = ? AND stopped_at = ? AND delivered_at IS NULL", examSetID, stoppedAt).
		Count(&pending).Error
	p.stopSawPersistence = stateErr == nil && eventErr == nil && !active && pending == 1
	return nil
}

func (*integrationOutboxProjector) RecordProjectionFailure(context.Context, uuid.UUID, error) error {
	return nil
}

func TestPostgresOutboxDispatchersClaimAttemptOnceAcrossReplicas(t *testing.T) {
	db := openOutboxDispatcherIntegrationDB(t)
	attempts := attemptrepo.NewPostgresRepository(db)
	lifecycle := examsetrepo.NewAdminRepository(db)
	projector := &integrationOutboxProjector{db: db}
	now := dispatcherIntegrationTime()
	insertDispatcherAttemptEvent(t, db, now)
	config := OutboxDispatcherConfig{Now: func() time.Time { return now }}
	dispatchers := []*OutboxDispatcher{
		NewOutboxDispatcher(attempts, lifecycle, projector, config),
		NewOutboxDispatcher(attempts, lifecycle, projector, config),
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(dispatchers))
	for _, dispatcher := range dispatchers {
		go func(d *OutboxDispatcher) {
			defer wg.Done()
			<-start
			_, _ = d.DrainOnce(t.Context())
		}(dispatcher)
	}
	close(start)
	wg.Wait()

	if projector.projectCalls != 1 {
		t.Fatalf("project calls = %d, want 1", projector.projectCalls)
	}
	var delivered int64
	if err := db.Table("leaderboard_attempt_projection_outbox").Where("delivered_at IS NOT NULL").Count(&delivered).Error; err != nil {
		t.Fatal(err)
	}
	if delivered != 1 {
		t.Fatalf("delivered rows = %d, want 1", delivered)
	}
}

func TestPostgresLifecycleCallbackSeesPersistedStopState(t *testing.T) {
	db := openOutboxDispatcherIntegrationDB(t)
	attempts := attemptrepo.NewPostgresRepository(db)
	lifecycle := examsetrepo.NewAdminRepository(db)
	projector := &integrationOutboxProjector{db: db}
	now := dispatcherIntegrationTime()
	examSetID := uuid.New()
	if err := db.Exec(`
		INSERT INTO exam_sets (id, status, is_active, created_at, updated_at)
		VALUES (?, 'published', true, ?, ?)
	`, examSetID, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.UpdateIsActive(t.Context(), examSetID, false); err != nil {
		t.Fatalf("UpdateIsActive() error = %v", err)
	}
	dispatcher := NewOutboxDispatcher(attempts, lifecycle, projector, OutboxDispatcherConfig{})
	if _, err := dispatcher.DrainOnce(t.Context()); err != nil {
		t.Fatalf("DrainOnce() error = %v", err)
	}
	if projector.stopCalls != 1 || !projector.stopSawPersistence {
		t.Fatalf("stop callback calls/persisted = %d/%v, want 1/true", projector.stopCalls, projector.stopSawPersistence)
	}
}

func openOutboxDispatcherIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LEADERBOARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEADERBOARD_POSTGRES_DSN is not set")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	schema := "dispatcher_task4_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dispatcherIntegrationDSN(dsn, schema)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error })
	statements := []string{
		`CREATE TABLE exam_sets (id uuid PRIMARY KEY, status text NOT NULL, is_active boolean NOT NULL, published_at timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL)`,
		`CREATE TABLE exam_attempts (id uuid PRIMARY KEY)`,
		`CREATE TABLE exam_tracks (id uuid PRIMARY KEY, code text NOT NULL)`,
		`CREATE TABLE exam_set_lifecycle_stop_events (
			exam_set_id uuid NOT NULL, stopped_at timestamptz NOT NULL, delivered_at timestamptz,
			claim_token uuid, claimed_at timestamptz, delivery_attempts int NOT NULL DEFAULT 0,
			next_attempt_at timestamptz NOT NULL DEFAULT now(), last_error text,
			created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (exam_set_id, stopped_at)
		)`,
		`CREATE TABLE leaderboard_attempt_projection_outbox (
			attempt_id uuid PRIMARY KEY, user_id uuid NOT NULL, exam_set_id uuid NOT NULL,
			exam_track_id uuid NOT NULL, track_code text NOT NULL, submitted_at timestamptz NOT NULL,
			points numeric(6,1) NOT NULL, duration_seconds int NOT NULL, delivered_at timestamptz,
			claim_token uuid, claimed_at timestamptz, delivery_attempts int NOT NULL DEFAULT 0,
			next_attempt_at timestamptz NOT NULL DEFAULT now(), last_error text,
			created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func insertDispatcherAttemptEvent(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO leaderboard_attempt_projection_outbox (
			attempt_id, user_id, exam_set_id, exam_track_id, track_code,
			submitted_at, points, duration_seconds, next_attempt_at
		) VALUES (?, ?, ?, ?, 'police', ?, 80, 600, ?)
	`, uuid.New(), uuid.New(), uuid.New(), uuid.New(), now, now).Error; err != nil {
		t.Fatal(err)
	}
}

func dispatcherIntegrationTime() time.Time {
	return time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
}

func dispatcherIntegrationDSN(dsn, schema string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return fmt.Sprintf("%s search_path=%s", dsn, schema)
}
