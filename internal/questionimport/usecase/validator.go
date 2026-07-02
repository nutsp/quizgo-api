package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	qdomain "virtual-exam-api/internal/question/domain"
	"virtual-exam-api/internal/questionimport/domain"
	"virtual-exam-api/internal/questionimport/mathconvert"
	"virtual-exam-api/internal/questionimport/zipimages"
	tagrepo "virtual-exam-api/internal/questiontag/repository"
)

type subjectLookup interface {
	FindByCode(ctx context.Context, code string) (*qdomain.Subject, error)
}

func validateRows(
	ctx context.Context,
	rows []domain.ImportQuestionRow,
	subjects subjectLookup,
	tags tagrepo.TagAdminRepository,
	existsFn func(ctx context.Context, text string) (bool, error),
	images map[string][]byte,
) []domain.ImportPreviewRow {
	textCounts := make(map[string]int)
	for _, row := range rows {
		key := strings.TrimSpace(row.QuestionText)
		if key != "" {
			textCounts[key]++
		}
	}

	preview := make([]domain.ImportPreviewRow, len(rows))
	for i, row := range rows {
		preview[i] = validateRow(ctx, row, subjects, tags, existsFn, textCounts, images)
	}
	return preview
}

func validateRow(
	ctx context.Context,
	row domain.ImportQuestionRow,
	subjects subjectLookup,
	tags tagrepo.TagAdminRepository,
	existsFn func(ctx context.Context, text string) (bool, error),
	textCounts map[string]int,
	images map[string][]byte,
) domain.ImportPreviewRow {
	errs := []string{}
	warns := []string{}

	data := normalizeImportRow(row)

	if data.SubjectCode == "" {
		errs = append(errs, "กรุณาระบุ subject_code")
	} else {
		subject, err := subjects.FindByCode(ctx, data.SubjectCode)
		if err != nil {
			errs = append(errs, "ไม่พบ subject_code นี้")
		} else if subject == nil {
			errs = append(errs, "ไม่พบ subject_code นี้")
		}
	}

	if data.QuestionText == "" && data.QuestionImage == "" {
		errs = append(errs, "กรุณาระบุข้อความคำถามหรือรูปภาพคำถาม")
	} else if data.QuestionText != "" {
		if utf8.RuneCountInString(data.QuestionText) < 5 {
			errs = append(errs, "คำถามสั้นเกินไป (อย่างน้อย 5 ตัวอักษร)")
		} else if utf8.RuneCountInString(data.QuestionText) < 10 {
			warns = append(warns, "คำถามสั้นมาก กรุณาตรวจสอบความถูกต้อง")
		}
	}

	validateChoiceContent := func(label, text, imageFile string) {
		if text == "" && imageFile == "" {
			errs = append(errs, "ตัวเลือก "+label+" ต้องมีข้อความหรือรูปภาพ")
		}
		if imageFile != "" {
			if _, ok := zipimages.LookupImage(images, imageFile); !ok {
				errs = append(errs, "ไม่พบไฟล์รูปภาพ: "+filepathBase(imageFile))
			}
		}
	}
	validateChoiceContent("ก", data.ChoiceA, data.ChoiceAImage)
	validateChoiceContent("ข", data.ChoiceB, data.ChoiceBImage)
	validateChoiceContent("ค", data.ChoiceC, data.ChoiceCImage)
	validateChoiceContent("ง", data.ChoiceD, data.ChoiceDImage)

	if data.QuestionImage != "" {
		if _, ok := zipimages.LookupImage(images, data.QuestionImage); !ok {
			errs = append(errs, "ไม่พบไฟล์รูปภาพ: "+filepathBase(data.QuestionImage))
		}
	}
	if data.ExplanationImage != "" {
		if _, ok := zipimages.LookupImage(images, data.ExplanationImage); !ok {
			errs = append(errs, "ไม่พบไฟล์รูปภาพ: "+filepathBase(data.ExplanationImage))
		}
	}

	normalized, ok := normalizeCorrectChoice(strings.TrimSpace(row.CorrectChoice))
	if !ok {
		errs = append(errs, "correct_choice ต้องเป็น A, B, C หรือ D")
	} else {
		data.CorrectChoice = normalized
	}

	if data.ContentFormat != "" && !qdomain.IsValidContentFormat(data.ContentFormat) {
		errs = append(errs, "content_format ไม่ถูกต้อง")
	}

	qt := strings.ToLower(strings.TrimSpace(data.QuestionType))
	if qt != "" && qt != "normal" && qt != "math" && qt != "image" {
		errs = append(errs, "question_type ไม่ถูกต้อง")
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

	if data.Tags != "" && tags != nil {
		codes := parseTagCodes(data.Tags)
		if len(codes) == 0 {
			errs = append(errs, "รูปแบบกลุ่มคำถาม (tags) ไม่ถูกต้อง")
		} else {
			found, err := tags.FindActiveByCodes(ctx, codes)
			if err != nil {
				errs = append(errs, "ไม่สามารถตรวจสอบกลุ่มคำถามได้")
			} else {
				foundCodes := make(map[string]bool, len(found))
				for _, t := range found {
					foundCodes[t.Code] = true
				}
				for _, code := range codes {
					if !foundCodes[code] {
						errs = append(errs, "ไม่พบกลุ่มคำถาม: "+code)
					}
				}
			}
		}
		data.Tags = strings.Join(parseTagCodes(data.Tags), "|")
	}

	if data.QuestionText != "" {
		if textCounts[data.QuestionText] > 1 {
			warns = append(warns, "พบคำถามซ้ำในไฟล์นี้")
		}
		if exists, err := existsFn(ctx, data.QuestionText); err == nil && exists {
			warns = append(warns, "พบคำถามนี้อยู่แล้วในระบบ")
		}
	}

	if mathconvert.ShouldConvert(data.QuestionType, data.ContentFormat) {
		data.QuestionText = mathconvert.ConvertSimpleMath(data.QuestionText)
		data.Explanation = mathconvert.ConvertSimpleMath(data.Explanation)
		data.ChoiceA = mathconvert.ConvertSimpleMath(data.ChoiceA)
		data.ChoiceB = mathconvert.ConvertSimpleMath(data.ChoiceB)
		data.ChoiceC = mathconvert.ConvertSimpleMath(data.ChoiceC)
		data.ChoiceD = mathconvert.ConvertSimpleMath(data.ChoiceD)
	}

	return domain.ImportPreviewRow{
		RowNumber: row.RowNumber,
		Data:      data,
		Valid:     len(errs) == 0,
		Errors:    errs,
		Warnings:  warns,
	}
}

func normalizeImportRow(row domain.ImportQuestionRow) domain.ImportQuestionRow {
	data := row
	data.SubjectCode = strings.ToLower(strings.TrimSpace(row.SubjectCode))
	data.QuestionType = strings.ToLower(strings.TrimSpace(row.QuestionType))
	data.ContentFormat = strings.ToLower(strings.TrimSpace(row.ContentFormat))
	data.QuestionText = strings.TrimSpace(row.QuestionText)
	data.QuestionImage = strings.TrimSpace(row.QuestionImage)
	data.ChoiceA = strings.TrimSpace(row.ChoiceA)
	data.ChoiceAImage = strings.TrimSpace(row.ChoiceAImage)
	data.ChoiceB = strings.TrimSpace(row.ChoiceB)
	data.ChoiceBImage = strings.TrimSpace(row.ChoiceBImage)
	data.ChoiceC = strings.TrimSpace(row.ChoiceC)
	data.ChoiceCImage = strings.TrimSpace(row.ChoiceCImage)
	data.ChoiceD = strings.TrimSpace(row.ChoiceD)
	data.ChoiceDImage = strings.TrimSpace(row.ChoiceDImage)
	data.Explanation = strings.TrimSpace(row.Explanation)
	data.ExplanationImage = strings.TrimSpace(row.ExplanationImage)
	data.Difficulty = strings.ToLower(strings.TrimSpace(row.Difficulty))
	data.Status = strings.ToLower(strings.TrimSpace(row.Status))
	data.Tags = strings.TrimSpace(row.Tags)

	if data.ContentFormat == "" {
		switch data.QuestionType {
		case "math":
			data.ContentFormat = qdomain.ContentFormatMarkdownMath
		default:
			data.ContentFormat = qdomain.ContentFormatPlain
		}
	}
	return data
}

func filepathBase(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
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

func parseTagCodes(raw string) []string {
	parts := strings.Split(raw, "|")
	seen := make(map[string]bool)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		code := strings.ToLower(strings.TrimSpace(p))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}
