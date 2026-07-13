package repository

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPostgresMarkAttemptTimeoutUsesDatabaseDeadlineAndFreezesDuration(t *testing.T) {
	db := openAttemptTimeoutIntegrationDB(t)
	repo := NewPostgresRepository(db)
	trackID := uuid.New()
	if err := db.Exec(`INSERT INTO exam_tracks (id, code) VALUES (?, 'police')`, trackID).Error; err != nil {
		t.Fatalf("insert track: %v", err)
	}
	futureAttemptID := uuid.New()
	if err := db.Exec(`
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status,
			started_at, expires_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, 'in_progress', CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP + interval '1 hour', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, futureAttemptID, uuid.New(), trackID, uuid.New()).Error; err != nil {
		t.Fatalf("insert attempt: %v", err)
	}
	changed, err := repo.MarkAttemptTimeout(t.Context(), futureAttemptID)
	if err != nil || changed {
		t.Fatalf("future MarkAttemptTimeout() = %t, %v, want false, nil", changed, err)
	}
	var futureStatus string
	var futureOutboxRows int64
	if err := db.Table("exam_attempts").Select("status").Where("id = ?", futureAttemptID).Scan(&futureStatus).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("leaderboard_attempt_projection_outbox").Where("attempt_id = ?", futureAttemptID).Count(&futureOutboxRows).Error; err != nil {
		t.Fatal(err)
	}
	if futureStatus != "in_progress" || futureOutboxRows != 0 {
		t.Fatalf("future timeout state/outbox = %s/%d, want in_progress/0", futureStatus, futureOutboxRows)
	}
	equalityAttemptID := uuid.New()
	equalityTx := db.Begin()
	if equalityTx.Error != nil {
		t.Fatal(equalityTx.Error)
	}
	defer equalityTx.Rollback()
	if err := equalityTx.Exec(`
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status,
			started_at, expires_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, 'in_progress', CURRENT_TIMESTAMP - interval '30 minutes',
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - interval '30 minutes', CURRENT_TIMESTAMP - interval '30 minutes')
	`, equalityAttemptID, uuid.New(), trackID, uuid.New()).Error; err != nil {
		t.Fatalf("insert equality attempt: %v", err)
	}
	equalityRepo := NewPostgresRepository(equalityTx)
	changed, err = equalityRepo.MarkAttemptTimeout(t.Context(), equalityAttemptID)
	if err != nil || !changed {
		t.Fatalf("equality MarkAttemptTimeout() = %t, %v, want true, nil", changed, err)
	}
	if err := equalityTx.Commit().Error; err != nil {
		t.Fatal(err)
	}

	attemptID := uuid.New()
	if err := db.Exec(`
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status,
			started_at, expires_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, 'in_progress', CURRENT_TIMESTAMP - interval '1 hour',
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - interval '1 hour', CURRENT_TIMESTAMP - interval '1 hour')
	`, attemptID, uuid.New(), trackID, uuid.New()).Error; err != nil {
		t.Fatalf("insert expired attempt: %v", err)
	}

	changed, err = repo.MarkAttemptTimeout(t.Context(), attemptID)
	if err != nil || !changed {
		t.Fatalf("first MarkAttemptTimeout() = %t, %v, want true, nil", changed, err)
	}
	changed, err = repo.MarkAttemptTimeout(t.Context(), attemptID)
	if err != nil || changed {
		t.Fatalf("second MarkAttemptTimeout() = %t, %v, want false, nil", changed, err)
	}
	var payload struct {
		AttemptDuration int `gorm:"column:attempt_duration"`
		OutboxDuration  int `gorm:"column:outbox_duration"`
	}
	if err := db.Raw(`
		SELECT ea.duration_seconds AS attempt_duration,
			o.duration_seconds AS outbox_duration
		FROM exam_attempts ea
		JOIN leaderboard_attempt_projection_outbox o ON o.attempt_id = ea.id
		WHERE ea.id = ?
	`, attemptID).Scan(&payload).Error; err != nil {
		t.Fatal(err)
	}
	if payload.AttemptDuration != 3600 || payload.OutboxDuration != 3600 {
		t.Fatalf("timeout durations = attempt:%d outbox:%d, want 3600/3600", payload.AttemptDuration, payload.OutboxDuration)
	}
}

func TestPostgresMarkAttemptTimeoutPreservesPersistedElapsedMode(t *testing.T) {
	db := openAttemptTimeoutIntegrationDB(t)
	if err := db.Exec(`ALTER TABLE exam_attempts ADD COLUMN timing_mode text NOT NULL DEFAULT 'countdown'`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(db)
	trackID := uuid.New()
	attemptID := uuid.New()
	if err := db.Exec(`INSERT INTO exam_tracks (id, code) VALUES (?, 'police')`, trackID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO exam_attempts (
			id, user_id, exam_track_id, exam_set_id, status, timing_mode,
			started_at, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'in_progress', 'elapsed',
			CURRENT_TIMESTAMP - interval '2 hours', CURRENT_TIMESTAMP - interval '1 hour',
			CURRENT_TIMESTAMP - interval '2 hours', CURRENT_TIMESTAMP - interval '2 hours')
	`, attemptID, uuid.New(), trackID, uuid.New()).Error; err != nil {
		t.Fatal(err)
	}

	changed, err := repo.MarkAttemptTimeout(t.Context(), attemptID)
	if err != nil || changed {
		t.Fatalf("elapsed MarkAttemptTimeout() = %t, %v, want false, nil", changed, err)
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
