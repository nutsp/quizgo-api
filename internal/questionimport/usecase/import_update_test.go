package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	qdomain "virtual-exam-api/internal/question/domain"
	"virtual-exam-api/internal/questionimport/domain"
	importrepo "virtual-exam-api/internal/questionimport/repository"
	tagdomain "virtual-exam-api/internal/questiontag/domain"
	tagrepo "virtual-exam-api/internal/questiontag/repository"
	subjectrepo "virtual-exam-api/internal/subject/repository"
)

func TestImportJobResponseUsesUploaderObject(t *testing.T) {
	userID := uuid.New()

	resp := toImportJobResponse(domain.ImportJob{
		ID:             uuid.New(),
		AdminUserID:    userID,
		AdminUserName:  "Admin",
		AdminUserEmail: "admin@example.com",
		Filename:       "questions.csv",
		Status:         domain.JobStatusPendingApproval,
		CreatedAt:      time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC),
	})

	if resp.UploadedBy.ID != userID.String() {
		t.Fatalf("expected uploader id %s, got %s", userID, resp.UploadedBy.ID)
	}
	if resp.UploadedBy.Name != "Admin" {
		t.Fatalf("expected uploader name Admin, got %s", resp.UploadedBy.Name)
	}
	if resp.UploadedBy.Email != "admin@example.com" {
		t.Fatalf("expected uploader email admin@example.com, got %s", resp.UploadedBy.Email)
	}
}

func TestUpdateImportItemRevalidatesRowAndBatchStats(t *testing.T) {
	ctx := context.Background()
	jobID := uuid.New()
	itemID := uuid.New()
	repo := &fakeImportRepository{
		job: domain.ImportJob{
			ID:          jobID,
			Status:      domain.JobStatusValidationFailed,
			TotalRows:   1,
			ValidRows:   0,
			InvalidRows: 1,
			CreatedAt:   time.Now().UTC(),
		},
		rows: []domain.ImportJobRow{{
			ID:           itemID,
			ImportJobID:  jobID,
			RowNumber:    2,
			SubjectCode:  "thai",
			QuestionText: "สั้น",
			ChoiceA:      "ก",
			ChoiceB:      "ข",
			ChoiceC:      "ค",
			ChoiceD:      "ง",
			Valid:        false,
			Errors:       []string{"คำถามสั้นเกินไป (อย่างน้อย 5 ตัวอักษร)"},
			CreatedAt:    time.Now().UTC(),
		}},
	}
	uc := NewUseCase(repo, fakeSubjectLookup{}, nil, fakeTagLookup{}, nil)

	updated, err := uc.UpdateItem(ctx, uuid.New(), UpdateImportItemInput{
		BatchID: jobID,
		ItemID:  itemID,
		ParsedData: ImportItemParsedData{
			SubjectCode:        "thai",
			QuestionGroupCodes: []string{"thai-reading-comprehension"},
			QuestionType:       "text",
			ContentFormat:      "plain_text",
			QuestionText:       "ข้อใดเป็นจุดประสงค์หลักของการอ่านจับใจความ?",
			Choices: []ImportItemChoiceInput{
				{Label: "A", Text: "อ่านเพื่อจับทุกคำในข้อความ"},
				{Label: "B", Text: "อ่านเพื่อหาความหมายหลักของข้อความ"},
				{Label: "C", Text: "อ่านเพื่อจับผิดการสะกดคำ"},
				{Label: "D", Text: "อ่านเพื่อแปลเป็นภาษาอังกฤษ"},
			},
			CorrectAnswer: "B",
			Explanation:   "การอ่านจับใจความคือการอ่านเพื่อสรุปสาระสำคัญหรือความหมายหลักของข้อความ",
		},
	})
	if err != nil {
		t.Fatalf("UpdateItem returned error: %v", err)
	}

	if !updated.IsValid {
		t.Fatalf("expected row to be valid, got errors: %v", updated.ValidationErrors)
	}
	if got := updated.ParsedData.CorrectAnswer; got != "B" {
		t.Fatalf("expected correct answer B, got %s", got)
	}
	if repo.job.Status != domain.JobStatusPendingApproval {
		t.Fatalf("expected job status pending_approval, got %s", repo.job.Status)
	}
	if repo.job.ValidRows != 1 || repo.job.InvalidRows != 0 {
		t.Fatalf("expected stats 1 valid / 0 invalid, got %d / %d", repo.job.ValidRows, repo.job.InvalidRows)
	}
}

type fakeImportRepository struct {
	job  domain.ImportJob
	rows []domain.ImportJobRow
}

func (r *fakeImportRepository) CreatePreview(context.Context, *domain.ImportJob, []domain.ImportJobRow) error {
	return nil
}

func (r *fakeImportRepository) FindJobByID(context.Context, uuid.UUID) (*domain.ImportJob, error) {
	job := r.job
	return &job, nil
}

func (r *fakeImportRepository) FindRowsByJobID(context.Context, uuid.UUID) ([]domain.ImportJobRow, error) {
	return append([]domain.ImportJobRow(nil), r.rows...), nil
}

func (r *fakeImportRepository) FindRowByID(context.Context, uuid.UUID, uuid.UUID) (*domain.ImportJobRow, error) {
	row := r.rows[0]
	return &row, nil
}

func (r *fakeImportRepository) ListPreviewRows(context.Context, uuid.UUID, importrepo.PreviewRowListFilter) ([]domain.ImportJobRow, int64, error) {
	return nil, 0, nil
}

func (r *fakeImportRepository) ListJobs(context.Context, importrepo.JobListFilter) ([]domain.ImportJob, int64, error) {
	return nil, 0, nil
}

func (r *fakeImportRepository) MarkImported(context.Context, uuid.UUID, int, int, int) error {
	return nil
}
func (r *fakeImportRepository) MarkRejected(context.Context, uuid.UUID) error { return nil }
func (r *fakeImportRepository) ExistsQuestionText(context.Context, string) (bool, error) {
	return false, nil
}
func (r *fakeImportRepository) RunInTransaction(context.Context, func(*gorm.DB) error) error {
	return nil
}
func (r *fakeImportRepository) MarkImportedTx(*gorm.DB, uuid.UUID, int, int, int) error { return nil }

func (r *fakeImportRepository) UpdateRowAndJobStats(
	_ context.Context,
	row domain.ImportJobRow,
	validRows int,
	invalidRows int,
	status string,
) error {
	r.rows[0] = row
	r.job.ValidRows = validRows
	r.job.InvalidRows = invalidRows
	r.job.Status = status
	return nil
}

type fakeSubjectLookup struct{}

func (fakeSubjectLookup) List(context.Context, subjectrepo.SubjectAdminFilter) ([]qdomain.Subject, int64, error) {
	return nil, 0, nil
}

func (fakeSubjectLookup) FindByID(context.Context, uuid.UUID) (*qdomain.Subject, error) {
	return nil, nil
}

func (fakeSubjectLookup) FindByCode(_ context.Context, code string) (*qdomain.Subject, error) {
	return &qdomain.Subject{ID: uuid.New(), Code: code, Name: code}, nil
}

func (fakeSubjectLookup) Create(context.Context, *qdomain.Subject) error { return nil }
func (fakeSubjectLookup) Update(context.Context, *qdomain.Subject) error { return nil }
func (fakeSubjectLookup) Delete(context.Context, uuid.UUID) error        { return nil }
func (fakeSubjectLookup) CountQuestions(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

type fakeTagLookup struct{}

func (fakeTagLookup) List(context.Context, tagrepo.TagAdminFilter) ([]tagdomain.QuestionTag, int64, error) {
	return nil, 0, nil
}

func (fakeTagLookup) FindByID(context.Context, uuid.UUID) (*tagdomain.QuestionTag, error) {
	return nil, nil
}
func (fakeTagLookup) FindByCode(context.Context, string) (*tagdomain.QuestionTag, error) {
	return nil, nil
}
func (fakeTagLookup) FindActiveByIDs(context.Context, []uuid.UUID) ([]tagdomain.QuestionTag, error) {
	return nil, nil
}

func (fakeTagLookup) FindActiveByCodes(_ context.Context, codes []string) ([]tagdomain.QuestionTag, error) {
	out := make([]tagdomain.QuestionTag, len(codes))
	for i, code := range codes {
		out[i] = tagdomain.QuestionTag{ID: uuid.New(), Code: code, IsActive: true}
	}
	return out, nil
}

func (fakeTagLookup) Create(context.Context, *tagdomain.QuestionTag) error        { return nil }
func (fakeTagLookup) Update(context.Context, *tagdomain.QuestionTag) error        { return nil }
func (fakeTagLookup) UpdateIsActive(context.Context, uuid.UUID, bool) error       { return nil }
func (fakeTagLookup) Delete(context.Context, uuid.UUID) error                     { return nil }
func (fakeTagLookup) Deactivate(context.Context, uuid.UUID) error                 { return nil }
func (fakeTagLookup) CountQuestions(context.Context, uuid.UUID) (int64, error)    { return 0, nil }
func (fakeTagLookup) ListActive(context.Context) ([]tagdomain.QuestionTag, error) { return nil, nil }
func (fakeTagLookup) LoadTagsForQuestions(context.Context, []uuid.UUID) (map[uuid.UUID][]tagdomain.TagRef, error) {
	return nil, nil
}
func (fakeTagLookup) ReplaceQuestionTagMappingsTx(*gorm.DB, uuid.UUID, []uuid.UUID) error { return nil }
