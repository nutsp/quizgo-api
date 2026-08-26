package repository

import (
	"reflect"
	"testing"
)

func TestExamAttemptModelStoresBlueprintVersionSnapshot(t *testing.T) {
	field, ok := reflect.TypeOf(ExamAttemptModel{}).FieldByName("BlueprintVersion")
	if !ok {
		t.Fatal("ExamAttemptModel.BlueprintVersion is missing")
	}
	if field.Type.Kind() != reflect.Int {
		t.Fatalf("BlueprintVersion type = %s, want int", field.Type)
	}
}
