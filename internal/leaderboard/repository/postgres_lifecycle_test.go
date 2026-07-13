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
	if !strings.Contains(findContainingExamSetIntervalSQL, "FOR UPDATE") {
		t.Error("containing interval lookup does not lock the selected row")
	}
	if !strings.Contains(closeContainingExamSetIntervalSQL, "joined_at < ?") {
		t.Error("join close SQL does not guard against a stale publish event")
	}
	if !strings.Contains(stopContainingExamSetIntervalsSQL, "joined_at <= ?") ||
		!strings.Contains(stopContainingExamSetIntervalsSQL, "stopped_at > ?") {
		t.Error("stop SQL does not shorten the interval containing the event time")
	}
	if strings.Contains(strings.ToUpper(insertSeasonExamSetIntervalSQL), "ON CONFLICT") {
		t.Error("join insert uses conflict suppression instead of returning unexpected interval conflicts")
	}
	if !strings.Contains(insertExamSetStopEventSQL, "ON CONFLICT") {
		t.Error("stop event SQL is not retry-idempotent")
	}
	if !strings.Contains(acquireSeasonProjectionLockSQL, "pg_advisory_xact_lock") {
		t.Error("projection SQL does not serialize rank mutations for the whole season")
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

func TestEnsureSeasonSQLUsesCurrentPublishedSetsAndAuthoritativeActivation(t *testing.T) {
	t.Parallel()

	for _, fragment := range []string{
		"FROM exam_sets",
		"status = ?",
		"is_active = true",
		"ORDER BY es.id",
	} {
		if !strings.Contains(listRolloverExamSetCandidatesSQL, fragment) {
			t.Errorf("rollover enrollment SQL missing %q", fragment)
		}
	}
	for _, staleFragment := range []string{"prior_season", "leaderboard_season_exam_sets", "ses.stopped_at IS NULL"} {
		if strings.Contains(listRolloverExamSetCandidatesSQL, staleFragment) {
			t.Errorf("rollover enrollment SQL still depends on stale prior interval state %q", staleFragment)
		}
	}
	for _, fragment := range []string{"published_at", "FOR SHARE"} {
		if !strings.Contains(findRolloverExamSetStateSQL, fragment) {
			t.Errorf("rollover state SQL missing %q", fragment)
		}
	}
	if !strings.Contains(listRolloverExamSetCandidatesSQL, "id <> ?") {
		t.Error("season creation does not exclude the newly published set")
	}
}

func TestLifecycleSQLUsesAuthoritativePublicationTimeAndRejectsOverlaps(t *testing.T) {
	t.Parallel()

	if !strings.Contains(findRolloverExamSetStateSQL, "published_at") {
		t.Error("rollover state does not use authoritative published_at")
	}
	for _, fragment := range []string{"ORDER BY joined_at DESC", "stopped_at", "FOR UPDATE"} {
		if !strings.Contains(findContainingExamSetIntervalSQL, fragment) {
			t.Errorf("containing interval SQL missing %q", fragment)
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
		"CREATE TABLE leaderboard_exam_set_stop_events",
		"EXCLUDE USING gist",
		"tstzrange(joined_at, stopped_at, '[)') WITH &&",
	} {
		if !strings.Contains(text, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
}

func TestProjectionTransactionUsesFreshReadCommittedState(t *testing.T) {
	t.Parallel()
	if projectionTxOptions.Isolation != sql.LevelReadCommitted {
		t.Fatalf("projection isolation = %v, want read committed", projectionTxOptions.Isolation)
	}
}
