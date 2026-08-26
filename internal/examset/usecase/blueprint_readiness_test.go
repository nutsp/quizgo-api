package usecase

import (
	"testing"

	"github.com/google/uuid"
	"virtual-exam-api/internal/examset/domain"
	qdomain "virtual-exam-api/internal/question/domain"
)

func TestBuildReadinessBlocksUnreviewedQuestions(t *testing.T) {
	set := &domain.ExamSet{
		ID:              uuid.New(),
		ExamTrackID:     uuid.New(),
		Code:            "trusted-set",
		Title:           "Trusted set",
		Description:     "Description",
		DurationMinutes: 60,
		TotalQuestions:  1,
		PassingScore:    60,
		Status:          domain.StatusDraft,
	}
	assigned := []qdomain.ExamSetQuestion{{
		QuestionID: uuid.New(),
		QuestionNo: 1,
		Question: &qdomain.Question{
			Status:       qdomain.StatusPublished,
			ReviewStatus: qdomain.ReviewStatusUnreviewed,
			IsActive:     true,
			Choices: []qdomain.Choice{
				{ChoiceKey: qdomain.ChoiceA, IsCorrect: true},
				{ChoiceKey: qdomain.ChoiceB},
				{ChoiceKey: qdomain.ChoiceC},
				{ChoiceKey: qdomain.ChoiceD},
			},
		},
	}}

	result := buildReadiness(set, assigned)
	for _, check := range result.Checks {
		if check.Key == "questions_are_reviewed" {
			if check.Passed {
				t.Fatal("unreviewed question must block readiness")
			}
			return
		}
	}
	t.Fatal("questions_are_reviewed check is missing")
}
