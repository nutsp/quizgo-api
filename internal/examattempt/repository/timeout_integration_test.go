package repository

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPostgresMarkAttemptTimeoutReportsTransitionOnce(t *testing.T) {
	db := openAttemptTimeoutIntegrationDB(t)
	repo := NewPostgresRepository(db)
	attemptID := uuid.New()
	trackID := uuid.New()
	startedAt := time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO exam_tracks (id, code) VALUES (?, 'police')`, trackID).Error; err != nil {
		t.Fatalf("insert track: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status,
			started_at, expires_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, 'in_progress', ?, ?, ?, ?)
	`, attemptID, uuid.New(), trackID, uuid.New(), startedAt, startedAt.Add(time.Hour), startedAt, startedAt).Error; err != nil {
		t.Fatalf("insert attempt: %v", err)
	}

	changed, err := repo.MarkAttemptTimeout(t.Context(), attemptID)
	if err != nil || !changed {
		t.Fatalf("first MarkAttemptTimeout() = %t, %v, want true, nil", changed, err)
	}
	changed, err = repo.MarkAttemptTimeout(t.Context(), attemptID)
	if err != nil || changed {
		t.Fatalf("second MarkAttemptTimeout() = %t, %v, want false, nil", changed, err)
	}
}

func openAttemptTimeoutIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LEADERBOARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEADERBOARD_POSTGRES_DSN is not set")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := "attempt_task4_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	db, err := gorm.Open(postgres.Open(attemptTimeoutDSN(dsn, schema)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
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
	if err := db.Exec(`CREATE TABLE exam_attempts (
		id uuid PRIMARY KEY,
		user_id uuid,
		exam_track_id uuid,
		exam_set_id uuid,
		status varchar(50) NOT NULL,
		started_at timestamptz NOT NULL,
		submitted_at timestamptz,
		expires_at timestamptz NOT NULL,
		duration_seconds int,
		score numeric(10,2) DEFAULT 0,
		total_score numeric(10,2) DEFAULT 0,
		score_percent numeric(10,2) DEFAULT 0,
		correct_count int DEFAULT 0,
		wrong_count int DEFAULT 0,
		unanswered_count int DEFAULT 0,
		created_at timestamptz NOT NULL,
		updated_at timestamptz NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create attempts table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE exam_tracks (id uuid PRIMARY KEY, code text NOT NULL)`).Error; err != nil {
		t.Fatalf("create tracks table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE leaderboard_attempt_projection_outbox (
		attempt_id uuid PRIMARY KEY,
		user_id uuid,
		exam_set_id uuid,
		exam_track_id uuid,
		track_code text,
		submitted_at timestamptz,
		points numeric(6,1),
		duration_seconds int
	)`).Error; err != nil {
		t.Fatalf("create projection outbox: %v", err)
	}
	return db
}

func attemptTimeoutDSN(dsn, schema string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return fmt.Sprintf("%s search_path=%s", dsn, schema)
}
