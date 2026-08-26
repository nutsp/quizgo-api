package repository

import (
	"strings"
	"testing"
)

func TestTrackLookupSelectsBlueprintReadinessFields(t *testing.T) {
	for _, field := range []string{"blueprint_status", "blueprint_duration_minutes"} {
		if !strings.Contains(trackSelectColumns, field) {
			t.Fatalf("track select columns missing %q", field)
		}
	}
}

func TestReadinessEvidenceCountsOnlyAnswersMatchingEachBlueprintSection(t *testing.T) {
	for _, fragment := range []string{
		"FILTER (WHERE q.id IS NOT NULL)",
		"ans.is_correct = true AND q.id IS NOT NULL",
	} {
		if !strings.Contains(readinessSectionsQuery, fragment) {
			t.Fatalf("readiness section query missing %q", fragment)
		}
	}
}
