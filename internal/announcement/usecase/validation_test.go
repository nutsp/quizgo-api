package usecase

import (
	"testing"

	"virtual-exam-api/internal/announcement/domain"
)

func TestValidateInputRequiresScheduleTrackAndDate(t *testing.T) {
	input := MutationInput{Title: "สนามสอบใกล้ถึง", Slug: "exam-soon", Type: domain.TypeExamSchedule, IsActive: true}
	if err := ValidateInput(input); err == nil {
		t.Fatal("expected schedule validation error")
	}
}

func TestValidateInputAcceptsInternalAndHTTPSCTA(t *testing.T) {
	for _, ctaURL := range []string{"/exams/gpor", "https://quizgo.example/exams/gpor"} {
		input := MutationInput{Title: "ประกาศ", Slug: "announcement", Type: domain.TypeGeneral, IsActive: true, CTAURL: ctaURL}
		if err := ValidateInput(input); err != nil {
			t.Fatalf("ValidateInput(%q) error = %v", ctaURL, err)
		}
	}
	input := MutationInput{Title: "ประกาศ", Slug: "announcement", Type: domain.TypeGeneral, IsActive: true, CTAURL: "//evil.example"}
	if err := ValidateInput(input); err == nil {
		t.Fatal("protocol-relative CTA URL should be rejected")
	}
}
