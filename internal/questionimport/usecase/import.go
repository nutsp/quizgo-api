package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/apperrors"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
	"virtual-exam-api/internal/questionimport/domain"
	importrepo "virtual-exam-api/internal/questionimport/repository"
	"virtual-exam-api/internal/questionimport/parser"
	subjectrepo "virtual-exam-api/internal/subject/repository"
)

const templateCSV = `subject_code,question_text,choice_a,choice_b,choice_c,choice_d,correct_choice,explanation,difficulty,status
law,"ข้อใดเป็นหนังสือราชการภายนอก","บันทึกข้อความ","หนังสือภายนอก","หนังสือสั่งการ","หนังสือประชาสัมพันธ์","B","หนังสือภายนอกใช้สำหรับติดต่อระหว่างส่วนราชการ",medium,published
math,"5 + 7 เท่ากับข้อใด","10","11","12","13","C","5 + 7 = 12",easy,published
`

type UseCase struct {
	imports   importrepo.Repository
	subjects  subjectrepo.SubjectAdminRepository
	questions questionrepo.QuestionAdminRepository
}

func NewUseCase(
	imports importrepo.Repository,
	subjects subjectrepo.SubjectAdminRepository,
	questions questionrepo.QuestionAdminRepository,
) *UseCase {
	return &UseCase{
		imports:   imports,
		subjects:  subjects,
		questions: questions,
	}
}

func (uc *UseCase) TemplateCSV() []byte {
	return []byte(templateCSV)
}

func (uc *UseCase) Preview(ctx context.Context, adminUserID uuid.UUID, filename string, data []byte) (*domain.ImportPreviewResult, error) {
	if len(data) == 0 {
		return nil, apperrors.New("EMPTY_FILE", "ไฟล์ว่างเปล่า", 400)
	}
	if len(data) > domain.MaxFileSize {
		return nil, apperrors.New("FILE_TOO_LARGE", "ไฟล์มีขนาดใหญ่เกินไป (สูงสุด 5MB)", 400)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".csv" && ext != ".xlsx" {
		return nil, apperrors.New("INVALID_FILE_TYPE", "รองรับเฉพาะไฟล์ .csv และ .xlsx", 400)
	}

	parsed, err := parser.Parse(filename, data)
	if err != nil {
		return nil, apperrors.New("PARSE_ERROR", err.Error(), 400)
	}

	previewRows := validateRows(ctx, parsed.Rows, uc.subjects, uc.imports.ExistsQuestionText)

	validCount := 0
	invalidCount := 0
	for _, row := range previewRows {
		if row.Valid {
			validCount++
		} else {
			invalidCount++
		}
	}

	jobID := uuid.New()
	now := time.Now().UTC()
	job := &domain.ImportJob{
		ID:          jobID,
		AdminUserID: adminUserID,
		Filename:    filename,
		Status:      domain.JobStatusPreview,
		TotalRows:   len(previewRows),
		ValidRows:   validCount,
		InvalidRows: invalidCount,
		CreatedAt:   now,
	}

	dbRows := make([]domain.ImportJobRow, len(previewRows))
	for i, row := range previewRows {
		dbRows[i] = domain.ImportJobRow{
			ID:            uuid.New(),
			ImportJobID:   jobID,
			RowNumber:     row.RowNumber,
			SubjectCode:   row.Data.SubjectCode,
			QuestionText:  row.Data.QuestionText,
			ChoiceA:       row.Data.ChoiceA,
			ChoiceB:       row.Data.ChoiceB,
			ChoiceC:       row.Data.ChoiceC,
			ChoiceD:       row.Data.ChoiceD,
			CorrectChoice: row.Data.CorrectChoice,
			Explanation:   row.Data.Explanation,
			Difficulty:    row.Data.Difficulty,
			Status:        row.Data.Status,
			Valid:         row.Valid,
			Errors:        row.Errors,
			Warnings:      row.Warnings,
			CreatedAt:     now,
		}
	}

	if err := uc.imports.CreatePreview(ctx, job, dbRows); err != nil {
		return nil, fmt.Errorf("create preview: %w", err)
	}

	return &domain.ImportPreviewResult{
		ImportID:    jobID,
		Filename:    filename,
		TotalRows:   len(previewRows),
		ValidRows:   validCount,
		InvalidRows: invalidCount,
		Rows:        previewRows,
	}, nil
}

func (uc *UseCase) Confirm(ctx context.Context, adminUserID uuid.UUID, input domain.ImportConfirmInput) (*domain.ImportConfirmResult, error) {
	if input.ImportID == uuid.Nil {
		return nil, apperrors.ErrInvalidInput
	}

	job, err := uc.imports.FindJobByID(ctx, input.ImportID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, apperrors.ErrNotFound
	}

	if job.Status == domain.JobStatusImported {
		return &domain.ImportConfirmResult{
			ImportID:          job.ID,
			Status:            domain.JobStatusConfirmed,
			ImportedQuestions: job.ImportedQuestions,
			SkippedRows:       job.SkippedRows,
			FailedRows:        job.FailedRows,
		}, nil
	}

	rows, err := uc.imports.FindRowsByJobID(ctx, input.ImportID)
	if err != nil {
		return nil, err
	}

	hasInvalid := false
	for _, row := range rows {
		if !row.Valid {
			hasInvalid = true
			break
		}
	}
	if hasInvalid && !input.ImportOnlyValidRows {
		return nil, apperrors.New("INVALID_ROWS", "มีแถวที่ไม่ถูกต้อง กรุณาเลือกนำเข้าเฉพาะแถวที่ถูกต้อง", 400)
	}

	imported := 0
	skipped := 0
	for _, row := range rows {
		if !row.Valid {
			skipped++
		}
	}

	importErr := uc.imports.RunInTransaction(ctx, func(tx *gorm.DB) error {
		imported = 0
		for _, row := range rows {
			if !row.Valid {
				continue
			}
			subject, err := uc.subjects.FindByCode(ctx, row.SubjectCode)
			if err != nil || subject == nil {
				return apperrors.New("SUBJECT_NOT_FOUND", "ไม่พบหมวดวิชานี้ในระบบ", 400)
			}
			question := buildQuestion(subject.ID, row)
			if err := uc.questions.CreateWithChoicesTx(ctx, tx, question); err != nil {
				return err
			}
			imported++
		}
		return uc.imports.MarkImportedTx(tx, input.ImportID, imported, skipped, 0)
	})
	if importErr != nil {
		return nil, importErr
	}

	return &domain.ImportConfirmResult{
		ImportID:          input.ImportID,
		Status:            domain.JobStatusImported,
		ImportedQuestions: imported,
		SkippedRows:       skipped,
		FailedRows:        0,
	}, nil
}

func buildQuestion(subjectID uuid.UUID, row domain.ImportJobRow) *qdomain.Question {
	choices := []qdomain.Choice{
		{ChoiceKey: qdomain.ChoiceA, ChoiceLabel: qdomain.ValidChoiceKeys[qdomain.ChoiceA], ChoiceText: row.ChoiceA, IsCorrect: row.CorrectChoice == qdomain.ChoiceA},
		{ChoiceKey: qdomain.ChoiceB, ChoiceLabel: qdomain.ValidChoiceKeys[qdomain.ChoiceB], ChoiceText: row.ChoiceB, IsCorrect: row.CorrectChoice == qdomain.ChoiceB},
		{ChoiceKey: qdomain.ChoiceC, ChoiceLabel: qdomain.ValidChoiceKeys[qdomain.ChoiceC], ChoiceText: row.ChoiceC, IsCorrect: row.CorrectChoice == qdomain.ChoiceC},
		{ChoiceKey: qdomain.ChoiceD, ChoiceLabel: qdomain.ValidChoiceKeys[qdomain.ChoiceD], ChoiceText: row.ChoiceD, IsCorrect: row.CorrectChoice == qdomain.ChoiceD},
	}

	isActive := row.Status != qdomain.StatusArchived

	return &qdomain.Question{
		SubjectID:    subjectID,
		QuestionText: row.QuestionText,
		Explanation:  row.Explanation,
		Difficulty:   row.Difficulty,
		Status:       row.Status,
		IsActive:     isActive,
		Choices:      choices,
	}
}
