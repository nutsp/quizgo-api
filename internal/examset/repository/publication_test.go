package repository

import (
	"reflect"
	"strings"
	"testing"
)

func TestExamSetModelDeclaresAuthoritativePublicationTimestamp(t *testing.T) {
	field, ok := reflect.TypeOf(ExamSetModel{}).FieldByName("PublishedAt")
	if !ok {
		t.Fatal("ExamSetModel.PublishedAt is missing")
	}
	if field.Type.String() != "*time.Time" {
		t.Fatalf("PublishedAt type = %s, want *time.Time", field.Type)
	}
}

func TestApplicationReconcileBackfillsOnlyMissingPublishedState(t *testing.T) {
	for _, fragment := range []string{
		"published_at = COALESCE(updated_at, created_at)",
		"status = 'published'",
		"published_at IS NULL",
	} {
		if !strings.Contains(reconcilePublishedAtSQL, fragment) {
			t.Errorf("publication reconciliation SQL missing %q", fragment)
		}
	}
}
