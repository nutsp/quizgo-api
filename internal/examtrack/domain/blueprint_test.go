package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestBlueprintValidateForReview(t *testing.T) {
	subjectA := uuid.New()
	subjectB := uuid.New()

	valid := Blueprint{
		Version:         2,
		Status:          BlueprintStatusReviewed,
		QuestionCount:   100,
		DurationMinutes: 120,
		PassingScore:    60,
		Sections: []BlueprintSection{
			{SubjectID: subjectA, WeightPercent: 40},
			{SubjectID: subjectB, WeightPercent: 60},
		},
	}
	if err := valid.ValidateForReview(); err != nil {
		t.Fatalf("valid blueprint returned error: %v", err)
	}

	invalidWeight := valid
	invalidWeight.Sections = []BlueprintSection{{SubjectID: subjectA, WeightPercent: 80}}
	if err := invalidWeight.ValidateForReview(); err == nil {
		t.Fatal("expected section weights that do not total 100 to fail")
	}

	invalidEvidence := valid
	invalidEvidence.QuestionCount = 0
	if err := invalidEvidence.ValidateForReview(); err == nil {
		t.Fatal("expected missing question count to fail")
	}
}
