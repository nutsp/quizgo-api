package seed

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
	userrepo "virtual-exam-api/internal/user/repository"
)

func TestPostgresFreshAutoMigrateAndSeedSetsPublicationTimestamp(t *testing.T) {
	dsn := os.Getenv("LEADERBOARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEADERBOARD_POSTGRES_DSN is not set")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	schemaName := "leaderboard_seed_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create seed integration schema: %v", err)
	}
	t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA IF EXISTS " + schemaName + " CASCADE").Error })

	testDB, err := gorm.Open(
		postgres.Open(seedDSNWithSearchPath(dsn, schemaName)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open schema-scoped PostgreSQL connection: %v", err)
	}
	if err := testDB.AutoMigrate(
		&userrepo.UserModel{},
		&trackrepo.ExamTrackModel{},
		&examsetrepo.ExamSetModel{},
		&questionrepo.SubjectModel{},
		&questionrepo.QuestionModel{},
		&questionrepo.ChoiceModel{},
		&questionrepo.ExamSetQuestionModel{},
		&attemptrepo.ExamAttemptModel{},
		&attemptrepo.ExamAnswerModel{},
		&leaderboardrepo.SeasonModel{},
		&leaderboardrepo.ExamSetStopEventModel{},
		&leaderboardrepo.SeasonExamSetModel{},
		&leaderboardrepo.ScoreModel{},
		&leaderboardrepo.EntryModel{},
		&leaderboardrepo.AwardModel{},
		&leaderboardrepo.ProjectionFailureModel{},
	); err != nil {
		t.Fatalf("fresh AutoMigrate: %v", err)
	}

	var stopEventColumn struct {
		IsNullable    string
		ColumnDefault *string
	}
	if err := testDB.Raw(`
		SELECT is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'leaderboard_exam_set_stop_events'
		  AND column_name = 'created_at'
	`).Scan(&stopEventColumn).Error; err != nil {
		t.Fatalf("inspect stop-event created_at column: %v", err)
	}
	if stopEventColumn.IsNullable != "NO" {
		t.Errorf("stop-event created_at nullable = %q, want NO", stopEventColumn.IsNullable)
	}
	if stopEventColumn.ColumnDefault == nil || !strings.Contains(*stopEventColumn.ColumnDefault, "now()") {
		t.Errorf("stop-event created_at default = %v, want now()", stopEventColumn.ColumnDefault)
	}
	if err := Run(t.Context(), testDB); err != nil {
		t.Fatalf("seed fresh database: %v", err)
	}

	var missing int64
	if err := testDB.Model(&examsetrepo.ExamSetModel{}).
		Where("status = 'published' AND published_at IS NULL").
		Count(&missing).Error; err != nil {
		t.Fatalf("count seeded publication timestamps: %v", err)
	}
	if missing != 0 {
		t.Fatalf("seeded published sets without published_at = %d, want 0", missing)
	}
}

func seedDSNWithSearchPath(dsn, schemaName string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schemaName)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn + " search_path=" + schemaName
}
