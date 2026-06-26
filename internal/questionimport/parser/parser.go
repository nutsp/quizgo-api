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
		RowNumber:     rowNum,
		SubjectCode:   get("subject_code"),
		QuestionText:  get("question_text"),
		ChoiceA:       get("choice_a"),
		ChoiceB:       get("choice_b"),
		ChoiceC:       get("choice_c"),
		ChoiceD:       get("choice_d"),
		CorrectChoice: get("correct_choice"),
		Explanation:   get("explanation"),
		Difficulty:    get("difficulty"),
		Status:        get("status"),
	}
}

func buildColumnIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[normalizeHeader(h)] = i
	}
	return idx
}
