package repository

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func TestLifecycleSQLSerializesTransitionsAndGuardsEventTime(t *testing.T) {
	t.Parallel()

	if !strings.Contains(acquireExamSetTransitionLockSQL, "pg_advisory_xact_lock") {
		t.Error("lifecycle lock SQL does not acquire a transaction advisory lock")
	}
	if !strings.Contains(findOpenExamSetIntervalSQL, "FOR UPDATE") {
		t.Error("open interval lookup does not lock the selected row")
	}
	if !strings.Contains(closeOpenExamSetIntervalSQL, "joined_at < ?") {
		t.Error("join close SQL does not guard against a stale publish event")
	}
	if !strings.Contains(stopOpenExamSetIntervalsSQL, "joined_at <= ?") {
		t.Error("stop SQL does not guard against closing a newer interval")
	}
	if strings.Contains(strings.ToUpper(insertSeasonExamSetIntervalSQL), "ON CONFLICT") {
		t.Error("join insert uses conflict suppression instead of returning unexpected interval conflicts")
	}
	if !strings.Contains(acquireSeasonUserProjectionLockSQL, "pg_advisory_xact_lock") {
		t.Error("projection SQL does not serialize score and aggregate writes per season/user")
	}
	for _, fragment := range []string{"status", "is_active", "published_at", "FOR SHARE"} {
		if !strings.Contains(findExamSetPublicationStateForShareSQL, fragment) {
			t.Errorf("projection publication state SQL missing %q", fragment)
		}
	}
}

func TestApplicationReconcileRestoresIntervalOverlapConstraint(t *testing.T) {
	t.Parallel()
	joined := strings.Join(reconcileLifecycleSchemaSQL, "\n")
	for _, fragment := range []string{
		"CREATE EXTENSION IF NOT EXISTS btree_gist",
		"leaderboard_season_exam_sets_no_overlap",
		"EXCLUDE USING gist",
		"tstzrange(joined_at, stopped_at, '[)') WITH &&",
	} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("application lifecycle reconciliation SQL missing %q", fragment)
		}
	}
}

func TestEnsureSeasonSQLUsesPriorIntervalsAndExplicitBootstrapState(t *testing.T) {
	t.Parallel()

	for _, fragment := range []string{
		"prior_season",
		"leaderboard_season_exam_sets",
		"ses.stopped_at IS NULL",
		"ORDER BY es.id",
	} {
		if !strings.Contains(listRolloverExamSetCandidatesSQL, fragment) {
			t.Errorf("rollover enrollment SQL missing %q", fragment)
		}
	}
	for _, fragment := range []string{
		"FROM exam_sets",
		"status = ?",
		"is_active = true",
		"ORDER BY id",
	} {
		if !strings.Contains(listBootstrapExamSetCandidatesSQL, fragment) {
			t.Errorf("bootstrap enrollment SQL missing %q", fragment)
		}
	}
	if !strings.Contains(findBootstrapExamSetStateSQL, "published_at") {
		t.Error("bootstrap enrollment does not derive an effective time from persisted set state")
	}
	for name, query := range map[string]string{
		"rollover":  listRolloverExamSetCandidatesSQL,
		"bootstrap": listBootstrapExamSetCandidatesSQL,
	} {
		if !strings.Contains(query, "id <> ?") {
			t.Errorf("%s season creation does not exclude the newly published set", name)
		}
	}
}

func TestLifecycleSQLUsesAuthoritativePublicationTimeAndRejectsOverlaps(t *testing.T) {
	t.Parallel()

	if !strings.Contains(findBootstrapExamSetStateSQL, "published_at") {
		t.Error("bootstrap state does not use authoritative published_at")
	}
	for _, fragment := range []string{"ORDER BY joined_at DESC", "stopped_at", "FOR UPDATE"} {
		if !strings.Contains(findLatestExamSetIntervalSQL, fragment) {
			t.Errorf("latest interval SQL missing %q", fragment)
		}
	}

	migration, err := os.ReadFile("../../../migrations/000023_monthly_leaderboards.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(migration)
	for _, fragment := range []string{
		"ADD COLUMN IF NOT EXISTS published_at",
		"status = 'published'",
		"EXCLUDE USING gist",
		"tstzrange(joined_at, stopped_at, '[)') WITH &&",
	} {
		if !strings.Contains(text, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
}

func TestProjectionTransactionUsesRepeatableReadSnapshot(t *testing.T) {
	t.Parallel()
	if projectionTxOptions.Isolation != sql.LevelRepeatableRead {
		t.Fatalf("projection isolation = %v, want repeatable read", projectionTxOptions.Isolation)
	}
}
