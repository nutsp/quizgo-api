package usecase

import (
	"testing"

	"github.com/google/uuid"
	esqdomain "virtual-exam-api/internal/examsetquestion/domain"
)

func TestApplyExamSetDifficultyReplacesRuleDifficulty(t *testing.T) {
	rules := []esqdomain.QuestionRule{
		{SubjectID: uuid.New(), Difficulty: "easy", Count: 10},
		{SubjectID: uuid.New(), Difficulty: "", Count: 20},
	}

	result := applyExamSetDifficulty(rules, "hard")

	for index, rule := range result {
		if rule.Difficulty != "hard" {
			t.Fatalf("rule %d: got difficulty %q, want hard", index, rule.Difficulty)
		}
	}
	if rules[0].Difficulty != "easy" || rules[1].Difficulty != "" {
		t.Fatal("expected input rules to remain unchanged")
	}
}
