package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	qdomain "virtual-exam-api/internal/question/domain"
	"virtual-exam-api/internal/questionimport/domain"
)

type subjectLookup interface {
	FindByCode(ctx context.Context, code string) (*qdomain.Subject, error)
}

func validateRows(ctx context.Context, rows []domain.ImportQuestionRow, subjects subjectLookup, existsFn func(ctx context.Context, text string) (bool, error)) []domain.ImportPreviewRow {
	textCounts := make(map[string]int)
	for _, row := range rows {
		key := strings.TrimSpace(row.QuestionText)
		if key != "" {
			textCounts[key]++
		}
	}

	preview := make([]domain.ImportPreviewRow, len(rows))
	for i, row := range rows {
		preview[i] = validateRow(ctx, row, subjects, existsFn, textCounts)
	}
	return preview
}

func validateRow(
	ctx context.Context,
	row domain.ImportQuestionRow,
	subjects subjectLookup,
	existsFn func(ctx context.Context, text string) (bool, error),
	textCounts map[string]int,
) domain.ImportPreviewRow {
	errs := []string{}
	warns := []string{}

	data := row
	data.SubjectCode = strings.ToLower(strings.TrimSpace(row.SubjectCode))
	data.QuestionText = strings.TrimSpace(row.QuestionText)
	data.ChoiceA = strings.TrimSpace(row.ChoiceA)
	data.ChoiceB = strings.TrimSpace(row.ChoiceB)
	data.ChoiceC = strings.TrimSpace(row.ChoiceC)
	data.ChoiceD = strings.TrimSpace(row.ChoiceD)
	data.Explanation = strings.TrimSpace(row.Explanation)
	data.Difficulty = strings.ToLower(strings.TrimSpace(row.Difficulty))
	data.Status = strings.ToLower(strings.TrimSpace(row.Status))

	if data.SubjectCode == "" {
		errs = append(errs, "กรุณาระบุหมวดวิชา (subject_code)")
	} else {
		subject, err := subjects.FindByCode(ctx, data.SubjectCode)
		if err != nil {
			errs = append(errs, "ไม่พบหมวดวิชานี้ในระบบ")
		} else if subject == nil {
			errs = append(errs, "ไม่พบหมวดวิชานี้ในระบบ")
		}
	}

	if data.QuestionText == "" {
		errs = append(errs, "กรุณาระบุคำถาม")
	} else if utf8.RuneCountInString(data.QuestionText) < 5 {
		errs = append(errs, "คำถามสั้นเกินไป (อย่างน้อย 5 ตัวอักษร)")
	} else if utf8.RuneCountInString(data.QuestionText) < 10 {
		warns = append(warns, "คำถามสั้นมาก กรุณาตรวจสอบความถูกต้อง")
	}

	if data.ChoiceA == "" {
		errs = append(errs, "กรุณาระบุตัวเลือก ก")
	}
	if data.ChoiceB == "" {
		errs = append(errs, "กรุณาระบุตัวเลือก ข")
	}
	if data.ChoiceC == "" {
		errs = append(errs, "กรุณาระบุตัวเลือก ค")
	}
	if data.ChoiceD == "" {
		errs = append(errs, "กรุณาระบุตัวเลือก ง")
	}

	normalized, ok := normalizeCorrectChoice(strings.TrimSpace(row.CorrectChoice))
	if !ok {
		errs = append(errs, "เฉลยต้องเป็น A, B, C, D หรือ ก, ข, ค, ง")
	} else {
		data.CorrectChoice = normalized
	}

	if data.Difficulty == "" {
		data.Difficulty = qdomain.DifficultyMedium
	} else if !isValidDifficulty(data.Difficulty) {
		errs = append(errs, "ระดับความยากไม่ถูกต้อง")
	}

	if data.Status == "" {
		data.Status = qdomain.StatusDraft
	} else if !isValidStatus(data.Status) {
		errs = append(errs, "สถานะไม่ถูกต้อง")
	}

	if data.Explanation == "" {
		warns = append(warns, "ยังไม่มีคำอธิบายเฉลย")
	}

	if data.QuestionText != "" {
		if textCounts[data.QuestionText] > 1 {
			warns = append(warns, "พบคำถามซ้ำในไฟล์นี้")
		}
		if exists, err := existsFn(ctx, data.QuestionText); err == nil && exists {
			warns = append(warns, "พบคำถามนี้อยู่แล้วในระบบ")
		}
	}

	return domain.ImportPreviewRow{
		RowNumber: row.RowNumber,
		Data:      data,
		Valid:     len(errs) == 0,
		Errors:    errs,
		Warnings:  warns,
	}
}

func normalizeCorrectChoice(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "A", "ก":
		return qdomain.ChoiceA, true
	case "B", "ข":
		return qdomain.ChoiceB, true
	case "C", "ค":
		return qdomain.ChoiceC, true
	case "D", "ง":
		return qdomain.ChoiceD, true
	default:
		return "", false
	}
}

func isValidDifficulty(d string) bool {
	return d == qdomain.DifficultyEasy || d == qdomain.DifficultyMedium || d == qdomain.DifficultyHard
}

func isValidStatus(s string) bool {
	return s == qdomain.StatusDraft || s == qdomain.StatusPublished || s == qdomain.StatusArchived
}
