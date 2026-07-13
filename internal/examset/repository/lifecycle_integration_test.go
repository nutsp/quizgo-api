package repository

import (
	"errors"
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
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
)

func TestLifecycleEventModelMatchesOutboxContract(t *testing.T) {
	if got := (LifecycleEventModel{}).TableName(); got != "exam_set_lifecycle_events" {
		t.Fatalf("TableName() = %q", got)
	}
}

func TestPostgresLifecycleStopTimestampSurvivesUnrelatedEdit(t *testing.T) {
	db := openExamSetLifecycleIntegrationDB(t)
	repo := NewAdminRepository(db)
	examSetID := uuid.New()
	base := time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC)
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO exam_sets (id, status, is_active, published_at, created_at, updated_at)
		VALUES (?, 'published', true, ?, ?, ?)
	`, examSetID, base, base, base)

	if err := repo.UpdateIsActive(t.Context(), examSetID, false); err != nil {
		t.Fatalf("UpdateIsActive() error = %v", err)
	}
	events, err := repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Now: base.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("ClaimLifecycleEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("pending events = %+v, want 1", events)
	}
	stoppedAt := events[0].EventAt

	later := base.Add(24 * time.Hour)
	mustExamSetLifecycleExec(t, db, `UPDATE exam_sets SET title = 'ordinary edit', updated_at = ? WHERE id = ?`, later, examSetID)
	if err := db.Exec(`UPDATE exam_set_lifecycle_events SET claim_token = NULL, claimed_at = NULL WHERE exam_set_id = ?`, examSetID).Error; err != nil {
		t.Fatal(err)
	}
	events, err = repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Now: later.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("ClaimLifecycleEvents() after edit error = %v", err)
	}
	if len(events) != 1 || !events[0].EventAt.Equal(stoppedAt) {
		t.Fatalf("pending event after edit = %+v, want durable %v", events, stoppedAt)
	}
	marked, err := repo.MarkLifecycleEventDelivered(t.Context(), events[0], later.Add(49*time.Hour))
	if err != nil || !marked {
		t.Fatalf("MarkLifecycleEventDelivered() = %t, %v", marked, err)
	}
	events, err = repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Now: later.Add(50 * time.Hour),
	})
	if err != nil || len(events) != 0 {
		t.Fatalf("pending events after delivery = %+v, %v", events, err)
	}
}

func TestPostgresPublishAndStopLifecycleEventsAreTransactionalAndOrdered(t *testing.T) {
	db := openExamSetLifecycleIntegrationDB(t)
	repo := NewAdminRepository(db)
	examSetID := uuid.New()
	trackID := uuid.New()
	base := time.Date(2026, 7, 14, 1, 30, 0, 0, time.UTC)
	mustExamSetLifecycleExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'draft', false, ?, ?)
	`, examSetID, trackID, base, base)

	if err := repo.UpdateStatus(t.Context(), examSetID, "published", true); err != nil {
		t.Fatalf("publish transition: %v", err)
	}
	if err := repo.UpdateIsActive(t.Context(), examSetID, false); err != nil {
		t.Fatalf("stop transition: %v", err)
	}

	claimNow := time.Now().UTC().Add(time.Hour)
	events, err := repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Limit: 10, Now: claimNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != LifecycleEventPublished {
		t.Fatalf("first claim = %+v, want only publish", events)
	}
	var publishedAt time.Time
	if err := db.Table("exam_sets").Select("published_at").Where("id = ?", examSetID).Scan(&publishedAt).Error; err != nil {
		t.Fatal(err)
	}
	if !events[0].EventAt.Equal(publishedAt) || events[0].ExamTrackID != trackID {
		t.Fatalf("publish event = %+v, persisted published_at = %v", events[0], publishedAt)
	}
	retryAt := claimNow.Add(2 * time.Hour)
	retried, err := repo.RetryLifecycleEvent(t.Context(), events[0], retryAt, errors.New("publish delivery failed"))
	if err != nil || !retried {
		t.Fatalf("retry publish = %t, %v", retried, err)
	}
	blocked, err := repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Limit: 10, Now: claimNow.Add(time.Hour),
	})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("claim before publish retry = %+v, %v, want stop blocked", blocked, err)
	}
	events, err = repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Limit: 10, Now: retryAt,
	})
	if err != nil || len(events) != 1 || events[0].EventType != LifecycleEventPublished {
		t.Fatalf("retry claim = %+v, %v, want publish", events, err)
	}
	marked, err := repo.MarkLifecycleEventDelivered(t.Context(), events[0], retryAt)
	if err != nil || !marked {
		t.Fatalf("mark publish = %t, %v", marked, err)
	}

	events, err = repo.ClaimLifecycleEvents(t.Context(), LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: &examSetID, Limit: 10, Now: retryAt.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != LifecycleEventStopped || events[0].EventAt.Before(publishedAt) {
		t.Fatalf("second claim = %+v, want later stop", events)
	}
}

func TestPostgresPublishRollsBackWhenLifecycleEventCannotPersist(t *testing.T) {
	db := openExamSetLifecycleIntegrationDB(t)
	repo := NewAdminRepository(db)
	examSetID := uuid.New()
	trackID := uuid.New()
	base := time.Date(2026, 7, 14, 1, 45, 0, 0, time.UTC)
	mustExamSetLifecycleExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'draft', false, ?, ?)
	`, examSetID, trackID, base, base)
	mustExamSetLifecycleExec(t, db, `DROP TABLE exam_set_lifecycle_events`)

	if err := repo.UpdateStatus(t.Context(), examSetID, "published", true); err == nil {
		t.Fatal("publish transition succeeded without lifecycle outbox")
	}
	var state struct {
		Status      string
		IsActive    bool
		PublishedAt *time.Time
	}
	if err := db.Table("exam_sets").Select("status", "is_active", "published_at").Where("id = ?", examSetID).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.Status != "draft" || state.IsActive || state.PublishedAt != nil {
		t.Fatalf("rolled back state = %+v", state)
	}
}

func TestPostgresHardDeleteAfterDeactivateIsNotSuppressed(t *testing.T) {
	db := openExamSetLifecycleIntegrationDB(t)
	repo := NewAdminRepository(db)
	examSetID := uuid.New()
	base := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO exam_sets (id, status, is_active, created_at, updated_at)
		VALUES (?, 'published', false, ?, ?)
	`, examSetID, base, base)

	deactivated, err := repo.Delete(t.Context(), examSetID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deactivated {
		t.Fatal("Delete() deactivated = true, want hard delete")
	}
	var count int64
	if err := db.Table("exam_sets").Where("id = ?", examSetID).Count(&count).Error; err != nil {
		t.Fatalf("count exam set: %v", err)
	}
	if count != 0 {
		t.Fatalf("exam set rows = %d, want 0", count)
	}
}

func TestPostgresDeletePreservesImmutableLeaderboardHistory(t *testing.T) {
	db := openExamSetLifecycleIntegrationDB(t)
	repo := NewAdminRepository(db)
	examSetID := uuid.New()
	trackID := uuid.New()
	seasonID := uuid.New()
	base := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	mustExamSetLifecycleExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, published_at, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?, ?)
	`, examSetID, trackID, base, base, base)
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, 2026, 7, ?, ?, 'active')
	`, seasonID, trackID, base.Add(-time.Hour), base.Add(30*24*time.Hour))
	mustExamSetLifecycleExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, base)

	deactivated, err := repo.Delete(t.Context(), examSetID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deactivated {
		t.Fatal("Delete() deactivated = false, want history-preserving soft delete")
	}
	var state struct {
		Status   string
		IsActive bool
	}
	if err := db.Table("exam_sets").Select("status", "is_active").Where("id = ?", examSetID).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.Status != "archived" || state.IsActive {
		t.Fatalf("preserved state = %s/%v, want archived/false", state.Status, state.IsActive)
	}
	var historyCount int64
	if err := db.Table("leaderboard_season_exam_sets").Where("exam_set_id = ?", examSetID).Count(&historyCount).Error; err != nil {
		t.Fatal(err)
	}
	if historyCount != 1 {
		t.Fatalf("history rows = %d, want 1", historyCount)
	}
}

func TestAutoMigrateOutboxModelsKeepPendingPartialIndexes(t *testing.T) {
	db := openExamSetLifecycleIntegrationDB(t)
	if err := db.Migrator().DropTable("leaderboard_attempt_projection_outbox", "exam_set_lifecycle_events"); err != nil {
		t.Fatalf("drop migrated outboxes: %v", err)
	}
	if err := db.AutoMigrate(&LifecycleEventModel{}, &attemptrepo.ProjectionOutboxModel{}); err != nil {
		t.Fatalf("AutoMigrate outboxes: %v", err)
	}
	for _, indexName := range []string{
		"exam_set_lifecycle_events_pending_idx",
		"leaderboard_attempt_projection_outbox_pending_idx",
	} {
		var indexDef string
		if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`, indexName).Scan(&indexDef).Error; err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(indexDef), "where (delivered_at is null)") {
			t.Fatalf("index %s = %q, want delivered_at partial predicate", indexName, indexDef)
		}
	}

	var constraints int64
	if err := db.Raw(`
		SELECT count(*)
		FROM pg_constraint c
		JOIN pg_class rel ON rel.oid = c.conrelid
		WHERE rel.relname = 'leaderboard_attempt_projection_outbox'
		  AND c.contype = 'f'
		  AND pg_get_constraintdef(c.oid) IN (
			'FOREIGN KEY (attempt_id) REFERENCES exam_attempts(id)',
			'FOREIGN KEY (user_id) REFERENCES users(id)',
			'FOREIGN KEY (exam_set_id) REFERENCES exam_sets(id)',
			'FOREIGN KEY (exam_track_id) REFERENCES exam_tracks(id)'
		  )
	`).Scan(&constraints).Error; err != nil {
		t.Fatal(err)
	}
	if constraints != 4 {
		t.Fatalf("projection outbox foreign keys = %d, want 4", constraints)
	}
}

func openExamSetLifecycleIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LEADERBOARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEADERBOARD_POSTGRES_DSN is not set")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	ensureExamSetLifecycleTestExtension(t, admin)
	schema := "examset_task4_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	db, err := gorm.Open(postgres.Open(examSetLifecycleDSN(dsn, schema)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open schema database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access schema sql.DB: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatalf("access admin sql.DB: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		_ = adminSQL.Close()
	})

	mustExamSetLifecycleExec(t, db, `CREATE TABLE users (id uuid PRIMARY KEY)`)
	mustExamSetLifecycleExec(t, db, `CREATE TABLE exam_tracks (id uuid PRIMARY KEY)`)
	mustExamSetLifecycleExec(t, db, `CREATE TABLE exam_sets (
		id uuid PRIMARY KEY,
		exam_track_id uuid,
		status varchar(50) NOT NULL,
		is_active boolean NOT NULL,
		published_at timestamptz,
		title text NOT NULL DEFAULT '',
		created_at timestamptz NOT NULL,
		updated_at timestamptz NOT NULL
	)`)
	mustExamSetLifecycleExec(t, db, `CREATE TABLE exam_attempts (id uuid PRIMARY KEY, exam_set_id uuid NOT NULL)`)

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration test")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(230026)`).Error; err != nil {
			return err
		}
		for _, migrationNumber := range []string{"000023_monthly_leaderboards", "000024_exam_set_lifecycle_stop_events", "000025_leaderboard_projection_dispatch", "000026_exam_set_lifecycle_events"} {
			migrationPath := filepath.Join(filepath.Dir(currentFile), "../../../migrations/"+migrationNumber+".up.sql")
			migration, err := os.ReadFile(migrationPath)
			if err != nil {
				return fmt.Errorf("read migration %s: %w", migrationNumber, err)
			}
			if err := tx.Exec(string(migration)).Error; err != nil {
				return fmt.Errorf("apply migration %s: %w", migrationNumber, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("apply lifecycle fixture migrations: %v", err)
	}
	return db
}

func ensureExamSetLifecycleTestExtension(t *testing.T, admin *gorm.DB) {
	t.Helper()
	if err := admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(230026)`).Error; err != nil {
			return err
		}
		return tx.Exec(`CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public`).Error
	}); err != nil {
		t.Fatalf("install stable test extension: %v", err)
	}
}

func examSetLifecycleDSN(dsn, schema string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return fmt.Sprintf("%s search_path=%s", dsn, schema)
}

func mustExamSetLifecycleExec(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("fixture SQL: %v", err)
	}
}
