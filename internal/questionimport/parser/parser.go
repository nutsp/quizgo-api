package parser

import (
	"fmt"
	"path/filepath"
	"strings"

	"virtual-exam-api/internal/questionimport/domain"
)

type ParseResult struct {
	Rows []domain.ImportQuestionRow
}

func Parse(filename string, data []byte) (*ParseResult, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return parseCSV(data)
	case ".xlsx":
		return parseXLSX(data)
	default:
		return nil, fmt.Errorf("รองรับเฉพาะไฟล์ .csv และ .xlsx")
	}
}

func normalizeHeader(h string) string {
	return strings.TrimSpace(strings.ToLower(h))
}

func validateHeaders(headers []string) error {
	headerSet := make(map[string]bool, len(headers))
	for _, h := range headers {
		headerSet[normalizeHeader(h)] = true
	}
	for _, col := range domain.RequiredColumns {
		if !headerSet[col] {
			return fmt.Errorf("ไม่พบคอลัมน์ %s", col)
		}
	}
	return nil
}

func rowFromMap(rowNum int, colIndex map[string]int, record []string) domain.ImportQuestionRow {
	get := func(col string) string {
		idx, ok := colIndex[col]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}
	return domain.ImportQuestionRow{
		RowNumber:        rowNum,
		SubjectCode:      get("subject_code"),
		Tags:             get("tags"),
		QuestionType:     get("question_type"),
		ContentFormat:    get("content_format"),
		QuestionText:     get("question_text"),
		QuestionImage:    get("question_image"),
		ChoiceA:          get("choice_a"),
		ChoiceAImage:     get("choice_a_image"),
		ChoiceB:          get("choice_b"),
		ChoiceBImage:     get("choice_b_image"),
		ChoiceC:          get("choice_c"),
		ChoiceCImage:     get("choice_c_image"),
		ChoiceD:          get("choice_d"),
		ChoiceDImage:     get("choice_d_image"),
		CorrectChoice:    get("correct_choice"),
		Explanation:      get("explanation"),
		ExplanationImage: get("explanation_image"),
		Difficulty:       get("difficulty"),
		Status:           get("status"),
	}
}

func buildColumnIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[normalizeHeader(h)] = i
	}
	return idx
}
