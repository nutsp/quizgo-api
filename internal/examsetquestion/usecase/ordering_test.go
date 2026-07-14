package usecase

import (
	"testing"

	"github.com/google/uuid"
)

func TestStableRuleReorderItemsGroupsByRuleAndPreservesInternalOrder(t *testing.T) {
	a1 := uuid.New()
	a2 := uuid.New()
	b1 := uuid.New()
	b2 := uuid.New()

	items := stableRuleReorderItems([]ruleGroupedQuestion{
		{questionID: b2, questionNo: 1, ruleIndex: 1},
		{questionID: a2, questionNo: 2, ruleIndex: 0},
		{questionID: b1, questionNo: 3, ruleIndex: 1},
		{questionID: a1, questionNo: 4, ruleIndex: 0},
	})

	want := []uuid.UUID{a2, a1, b2, b1}
	for index, item := range items {
		if item.QuestionID != want[index] {
			t.Fatalf("item %d: got %s, want %s", index, item.QuestionID, want[index])
		}
		if item.QuestionNo != index+1 {
			t.Fatalf("item %d: got question_no %d, want %d", index, item.QuestionNo, index+1)
		}
	}
}
