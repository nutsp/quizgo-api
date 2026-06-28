package domain

import "testing"

func TestAnswerSheetLayoutConfig_Validate(t *testing.T) {
	valid := DefaultAnswerSheetLayout()
	if err := valid.Validate(); err != nil {
		t.Fatalf("default layout should be valid: %v", err)
	}

	cases := []AnswerSheetLayoutConfig{
		{BlockColumns: 0, QuestionsPerBlock: 10, ChoiceLabelStyle: ChoiceLabelThai},
		{BlockColumns: 5, QuestionsPerBlock: 10, ChoiceLabelStyle: ChoiceLabelThai},
		{BlockColumns: 2, QuestionsPerBlock: 4, ChoiceLabelStyle: ChoiceLabelThai},
		{BlockColumns: 2, QuestionsPerBlock: 51, ChoiceLabelStyle: ChoiceLabelThai},
		{BlockColumns: 2, QuestionsPerBlock: 10, ChoiceLabelStyle: "invalid"},
	}
	for _, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("expected invalid layout: %+v", c)
		}
	}
}
