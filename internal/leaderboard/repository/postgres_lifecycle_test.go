package repository

import (
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
	for _, fragment := range []string{"status = ?", "is_active = true"} {
		if !strings.Contains(findPublishedActiveExamSetSQL, fragment) {
			t.Errorf("projection publication recheck SQL missing %q", fragment)
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
	if !strings.Contains(findBootstrapExamSetStateSQL, "updated_at") {
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
