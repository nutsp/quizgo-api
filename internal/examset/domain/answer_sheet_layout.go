package domain

import "fmt"

const (
	ChoiceLabelThai    = "thai"
	ChoiceLabelEnglish = "english"
)

type AnswerSheetLayoutConfig struct {
	BlockColumns      int    `json:"block_columns"`
	QuestionsPerBlock int    `json:"questions_per_block"`
	ChoiceLabelStyle  string `json:"choice_label_style"`
	ShowHeader        bool   `json:"show_header"`
	ShowInstructions  bool   `json:"show_instructions"`
	ShowCandidateInfo bool   `json:"show_candidate_info"`
}

func DefaultAnswerSheetLayout() AnswerSheetLayoutConfig {
	return AnswerSheetLayoutConfig{
		BlockColumns:      2,
		QuestionsPerBlock: 10,
		ChoiceLabelStyle:  ChoiceLabelThai,
		ShowHeader:        true,
		ShowInstructions:  true,
		ShowCandidateInfo: true,
	}
}

func (c AnswerSheetLayoutConfig) Validate() error {
	if c.BlockColumns < 1 || c.BlockColumns > 4 {
		return fmt.Errorf("block_columns out of range")
	}
	if c.QuestionsPerBlock < 5 || c.QuestionsPerBlock > 50 {
		return fmt.Errorf("questions_per_block out of range")
	}
	if c.ChoiceLabelStyle != ChoiceLabelThai && c.ChoiceLabelStyle != ChoiceLabelEnglish {
		return fmt.Errorf("invalid choice_label_style")
	}
	return nil
}

func NormalizeAnswerSheetLayout(c AnswerSheetLayoutConfig) AnswerSheetLayoutConfig {
	def := DefaultAnswerSheetLayout()
	if c.BlockColumns < 1 || c.BlockColumns > 4 {
		c.BlockColumns = def.BlockColumns
	}
	if c.QuestionsPerBlock < 5 || c.QuestionsPerBlock > 50 {
		c.QuestionsPerBlock = def.QuestionsPerBlock
	}
	if c.ChoiceLabelStyle != ChoiceLabelThai && c.ChoiceLabelStyle != ChoiceLabelEnglish {
		c.ChoiceLabelStyle = def.ChoiceLabelStyle
	}
	return c
}

func LayoutFromModel(
	blockColumns, questionsPerBlock int,
	choiceLabelStyle string,
	showHeader, showInstructions, showCandidateInfo bool,
) AnswerSheetLayoutConfig {
	return NormalizeAnswerSheetLayout(AnswerSheetLayoutConfig{
		BlockColumns:      blockColumns,
		QuestionsPerBlock: questionsPerBlock,
		ChoiceLabelStyle:  choiceLabelStyle,
		ShowHeader:        showHeader,
		ShowInstructions:  showInstructions,
		ShowCandidateInfo: showCandidateInfo,
	})
}
