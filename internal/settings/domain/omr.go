package domain

import (
	"fmt"
	"strings"
	"time"

	examsetdomain "virtual-exam-api/internal/examset/domain"
)

const OMRAnswerSheetKey = "omr_answer_sheet"

type OMRAnswerSheetSettings struct {
	ColumnsPerRow       int        `json:"columns_per_row"`
	QuestionsPerColumn  int        `json:"questions_per_column"`
	ChoiceLabels        []string   `json:"choice_labels"`
	ShowHeader          bool       `json:"show_header"`
	ShowInstructions    bool       `json:"show_instructions"`
	ShowExaminerInfo    bool       `json:"show_examiner_info"`
	HoldToAnswerMS      int        `json:"hold_to_answer_ms"`
	SoundEnabledDefault bool       `json:"sound_enabled_default"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

func DefaultOMRAnswerSheetSettings() OMRAnswerSheetSettings {
	return OMRAnswerSheetSettings{
		ColumnsPerRow:       4,
		QuestionsPerColumn:  5,
		ChoiceLabels:        []string{"ก", "ข", "ค", "ง"},
		ShowHeader:          true,
		ShowInstructions:    true,
		ShowExaminerInfo:    true,
		HoldToAnswerMS:      350,
		SoundEnabledDefault: true,
	}
}

func (s OMRAnswerSheetSettings) Validate() error {
	if s.ColumnsPerRow < 1 || s.ColumnsPerRow > 10 {
		return fmt.Errorf("columns_per_row out of range")
	}
	if s.QuestionsPerColumn < 1 || s.QuestionsPerColumn > 50 {
		return fmt.Errorf("questions_per_column out of range")
	}
	if len(s.ChoiceLabels) < 2 || len(s.ChoiceLabels) > 6 {
		return fmt.Errorf("choice_labels length out of range")
	}
	for _, label := range s.ChoiceLabels {
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("choice_labels contains empty label")
		}
	}
	if s.HoldToAnswerMS < 150 || s.HoldToAnswerMS > 1500 {
		return fmt.Errorf("hold_to_answer_ms out of range")
	}
	return nil
}

func NormalizeOMRAnswerSheetSettings(s OMRAnswerSheetSettings) OMRAnswerSheetSettings {
	if err := s.Validate(); err != nil {
		return DefaultOMRAnswerSheetSettings()
	}
	return s
}

func (s OMRAnswerSheetSettings) ToAnswerSheetLayout() examsetdomain.AnswerSheetLayoutConfig {
	style := examsetdomain.ChoiceLabelThai
	if len(s.ChoiceLabels) > 0 && s.ChoiceLabels[0] == "A" {
		style = examsetdomain.ChoiceLabelEnglish
	}
	blockColumns := s.ColumnsPerRow
	if blockColumns > 4 {
		blockColumns = 4
	}
	return examsetdomain.NormalizeAnswerSheetLayout(examsetdomain.AnswerSheetLayoutConfig{
		BlockColumns:      blockColumns,
		QuestionsPerBlock: s.QuestionsPerColumn,
		ChoiceLabelStyle:  style,
		ShowHeader:        s.ShowHeader,
		ShowInstructions:  s.ShowInstructions,
		ShowCandidateInfo: s.ShowExaminerInfo,
	})
}
