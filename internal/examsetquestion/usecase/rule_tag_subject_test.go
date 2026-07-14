package usecase

import (
	"testing"

	"github.com/google/uuid"
	tagdomain "virtual-exam-api/internal/questiontag/domain"
)

func TestValidateRuleTagSubjectAcceptsMatchingSubject(t *testing.T) {
	subjectID := uuid.New()
	tag := &tagdomain.QuestionTag{ID: uuid.New(), SubjectID: &subjectID, IsActive: true}

	if err := validateRuleTagSubject(subjectID, tag); err != nil {
		t.Fatalf("expected matching subject to pass, got %v", err)
	}
}

func TestValidateRuleTagSubjectRejectsDifferentSubject(t *testing.T) {
	subjectID := uuid.New()
	otherSubjectID := uuid.New()
	tag := &tagdomain.QuestionTag{ID: uuid.New(), SubjectID: &otherSubjectID, IsActive: true}

	if err := validateRuleTagSubject(subjectID, tag); err == nil {
		t.Fatal("expected a tag from another subject to be rejected")
	}
}

func TestValidateRuleTagSubjectRejectsGlobalTag(t *testing.T) {
	tag := &tagdomain.QuestionTag{ID: uuid.New(), SubjectID: nil, IsActive: true}

	if err := validateRuleTagSubject(uuid.New(), tag); err == nil {
		t.Fatal("expected a tag without subject to be rejected")
	}
}

func TestValidateRuleTagSubjectRejectsInactiveTag(t *testing.T) {
	subjectID := uuid.New()
	tag := &tagdomain.QuestionTag{ID: uuid.New(), SubjectID: &subjectID, IsActive: false}

	if err := validateRuleTagSubject(subjectID, tag); err == nil {
		t.Fatal("expected an inactive tag to be rejected")
	}
}
