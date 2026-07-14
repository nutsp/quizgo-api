package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"virtual-exam-api/internal/config"
)

func NewPostgres(cfg *config.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %v", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

func MustMigrate(db *gorm.DB, models ...any) {
	reconcileLegacyConstraints(db)
	if err := db.AutoMigrate(models...); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}
	reconcilePostMigrate(db)
}

func reconcilePostMigrate(db *gorm.DB) {
	statements := []string{
		`WITH ranked AS (
			SELECT id, ROW_NUMBER() OVER (PARTITION BY exam_set_id ORDER BY rule_order ASC, id ASC) AS next_order
			FROM exam_set_question_rules
		)
		UPDATE exam_set_question_rules AS rules
		SET rule_order = ranked.next_order
		FROM ranked
			WHERE rules.id = ranked.id AND rules.rule_order <> ranked.next_order`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_exam_set_question_rules_order ON exam_set_question_rules(exam_set_id, rule_order)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			log.Printf("post migrate reconcile (non-fatal): %v", err)
		}
	}
}

// reconcileLegacyConstraints drops PostgreSQL default unique constraint names
// (from SQL migrations) and stale GORM names so AutoMigrate can recreate them.
func reconcileLegacyConstraints(db *gorm.DB) {
	statements := []string{
		// PostgreSQL default names from inline UNIQUE
		`ALTER TABLE IF EXISTS users DROP CONSTRAINT IF EXISTS users_email_key`,
		`ALTER TABLE IF EXISTS exam_tracks DROP CONSTRAINT IF EXISTS exam_tracks_code_key`,
		`ALTER TABLE IF EXISTS exam_sets DROP CONSTRAINT IF EXISTS exam_sets_code_key`,
		`ALTER TABLE IF EXISTS subjects DROP CONSTRAINT IF EXISTS subjects_code_key`,
		`ALTER TABLE IF EXISTS exam_set_questions DROP CONSTRAINT IF EXISTS exam_set_questions_exam_set_id_question_no_key`,
		`ALTER TABLE IF EXISTS exam_set_questions DROP CONSTRAINT IF EXISTS exam_set_questions_exam_set_id_question_id_key`,
		`ALTER TABLE IF EXISTS exam_answers DROP CONSTRAINT IF EXISTS exam_answers_attempt_id_question_id_key`,
		// Stale GORM auto-generated names from earlier runs
		`ALTER TABLE IF EXISTS users DROP CONSTRAINT IF EXISTS uni_users_email`,
		`ALTER TABLE IF EXISTS exam_tracks DROP CONSTRAINT IF EXISTS uni_exam_tracks_code`,
		`ALTER TABLE IF EXISTS exam_sets DROP CONSTRAINT IF EXISTS uni_exam_sets_code`,
		`ALTER TABLE IF EXISTS subjects DROP CONSTRAINT IF EXISTS uni_subjects_code`,
		`ALTER TABLE IF EXISTS exam_set_questions DROP CONSTRAINT IF EXISTS uq_exam_set_no`,
		`ALTER TABLE IF EXISTS exam_set_questions DROP CONSTRAINT IF EXISTS uq_exam_set_question`,
		`ALTER TABLE IF EXISTS exam_answers DROP CONSTRAINT IF EXISTS uq_attempt_question`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			log.Printf("migrate reconcile (non-fatal): %v", err)
		}
	}
}
