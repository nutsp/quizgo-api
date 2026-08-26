package repository

import (
	"reflect"
	"strings"
	"testing"
)

func TestBlueprintSectionsUseNonNullDatabaseDefault(t *testing.T) {
	field, ok := reflect.TypeOf(ExamTrackModel{}).FieldByName("BlueprintSections")
	if !ok {
		t.Fatal("ExamTrackModel.BlueprintSections is missing")
	}
	tag := field.Tag.Get("gorm")
	for _, fragment := range []string{"not null", "default:'[]'"} {
		if !strings.Contains(tag, fragment) {
			t.Fatalf("BlueprintSections gorm tag %q missing %q", tag, fragment)
		}
	}
}

func TestToDomainNormalizesEmptyBlueprintSections(t *testing.T) {
	track := toDomain(&ExamTrackModel{})
	if track.Blueprint.Sections == nil {
		t.Fatal("Blueprint.Sections must serialize as [] instead of null")
	}
}
