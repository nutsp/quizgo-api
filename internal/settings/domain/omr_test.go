package domain

import "testing"

func TestOMRAnswerSheetSettingsValidate(t *testing.T) {
	valid := DefaultOMRAnswerSheetSettings()
	if err := valid.Validate(); err != nil {
		t.Fatalf("default OMR settings should be valid: %v", err)
	}

	cases := []OMRAnswerSheetSettings{
		{ColumnsPerRow: 0, QuestionsPerColumn: 5, ChoiceLabels: []string{"ก", "ข"}, HoldToAnswerMS: 350},
		{ColumnsPerRow: 4, QuestionsPerColumn: 0, ChoiceLabels: []string{"ก", "ข"}, HoldToAnswerMS: 350},
		{ColumnsPerRow: 4, QuestionsPerColumn: 5, ChoiceLabels: []string{"ก"}, HoldToAnswerMS: 350},
		{ColumnsPerRow: 4, QuestionsPerColumn: 5, ChoiceLabels: []string{"ก", ""}, HoldToAnswerMS: 350},
		{ColumnsPerRow: 4, QuestionsPerColumn: 5, ChoiceLabels: []string{"ก", "ข"}, HoldToAnswerMS: 149},
	}
	for _, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("expected invalid OMR settings: %+v", c)
		}
	}
}

func TestOMRAnswerSheetSettingsToAnswerSheetLayoutClampsColumns(t *testing.T) {
	settings := DefaultOMRAnswerSheetSettings()
	settings.ColumnsPerRow = 10
	layout := settings.ToAnswerSheetLayout()
	if layout.BlockColumns != 4 {
		t.Fatalf("expected renderer-safe 4 columns, got %d", layout.BlockColumns)
	}
	if layout.QuestionsPerBlock != settings.QuestionsPerColumn {
		t.Fatalf("expected questions per block from settings, got %d", layout.QuestionsPerBlock)
	}
}
