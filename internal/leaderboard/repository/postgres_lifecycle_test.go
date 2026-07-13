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
}

func TestEnsureSeasonSQLEnrollsActivePublishedSetsAtSeasonStart(t *testing.T) {
	t.Parallel()

	for _, fragment := range []string{
		"FROM exam_sets",
		"exam_track_id = ?",
		"status = ?",
		"is_active = true",
	} {
		if !strings.Contains(enrollSeasonExamSetsSQL, fragment) {
			t.Errorf("rollover enrollment SQL missing %q", fragment)
		}
	}
	if !strings.Contains(enrollSeasonExamSetsSQL, "SELECT gen_random_uuid(), ?, id, ?") {
		t.Error("rollover enrollment SQL does not join active sets at the supplied season start")
	}
	if strings.Contains(strings.ToUpper(enrollSeasonExamSetsSQL), "ON CONFLICT") {
		t.Error("rollover enrollment hides interval conflicts")
	}
	if !strings.Contains(enrollSeasonExamSetsForPublishSQL, "id <> ?") {
		t.Error("publish-time season creation does not exclude the newly published set from rollover")
	}
}
