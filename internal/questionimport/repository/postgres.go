package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/questionimport/domain"
)

type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *StringList) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("invalid type for StringList")
	}
	return json.Unmarshal(data, s)
}

type ImportJobModel struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	AdminUserID       uuid.UUID  `gorm:"type:uuid;not null"`
	Filename          string     `gorm:"not null"`
	Status            string     `gorm:"type:varchar(50);not null;default:preview"`
	TotalRows         int        `gorm:"not null;default:0"`
	ValidRows         int        `gorm:"not null;default:0"`
	InvalidRows       int        `gorm:"not null;default:0"`
	ImportedQuestions int        `gorm:"not null;default:0"`
	SkippedRows       int        `gorm:"not null;default:0"`
	FailedRows        int        `gorm:"not null;default:0"`
	CreatedAt         time.Time  `gorm:"not null"`
	ConfirmedAt       *time.Time
}

func (ImportJobModel) TableName() string { return "question_import_jobs" }

type ImportRowModel struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ImportJobID         uuid.UUID  `gorm:"type:uuid;not null;index"`
	RowNumber           int        `gorm:"not null"`
	SubjectCode         string     `gorm:"type:varchar(100)"`
	Tags                string     `gorm:"type:varchar(500)"`
	QuestionType        string     `gorm:"type:varchar(30)"`
	ContentFormat       string     `gorm:"type:varchar(30)"`
	QuestionText        string     `gorm:"type:text"`
	QuestionImage       string     `gorm:"type:text"`
	QuestionImageURL    string     `gorm:"type:text"`
	ChoiceA             string     `gorm:"type:text"`
	ChoiceAImage        string     `gorm:"type:text"`
	ChoiceAImageURL     string     `gorm:"type:text"`
	ChoiceB             string     `gorm:"type:text"`
	ChoiceBImage        string     `gorm:"type:text"`
	ChoiceBImageURL     string     `gorm:"type:text"`
	ChoiceC             string     `gorm:"type:text"`
	ChoiceCImage        string     `gorm:"type:text"`
	ChoiceCImageURL     string     `gorm:"type:text"`
	ChoiceD             string     `gorm:"type:text"`
	ChoiceDImage        string     `gorm:"type:text"`
	ChoiceDImageURL     string     `gorm:"type:text"`
	CorrectChoice       string     `gorm:"type:varchar(10)"`
	Explanation         string     `gorm:"type:text"`
	ExplanationImage    string     `gorm:"type:text"`
	ExplanationImageURL string     `gorm:"type:text"`
	Difficulty          string     `gorm:"type:varchar(50)"`
	Status              string     `gorm:"type:varchar(50)"`
	Valid               bool       `gorm:"not null;default:false"`
	Errors              StringList `gorm:"type:jsonb;not null;default:'[]'"`
	Warnings            StringList `gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt           time.Time  `gorm:"not null"`
}

func (ImportRowModel) TableName() string { return "question_import_rows" }

type JobListFilter struct {
	Query    string
	Status   string
	DateFrom *time.Time
	DateTo   *time.Time
	Page     int
	Limit    int
	Sort     string
	Order    string
}

var importJobSortColumns = map[string]string{
	"created_at":   "created_at",
	"confirmed_at": "confirmed_at",
	"filename":     "filename",
	"status":       "status",
}

type Repository interface {
	CreatePreview(ctx context.Context, job *domain.ImportJob, rows []domain.ImportJobRow) error
	FindJobByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error)
	FindRowsByJobID(ctx context.Context, jobID uuid.UUID) ([]domain.ImportJobRow, error)
	ListJobs(ctx context.Context, filter JobListFilter) ([]domain.ImportJob, int64, error)
	MarkImported(ctx context.Context, jobID uuid.UUID, imported, skipped, failed int) error
	ExistsQuestionText(ctx context.Context, text string) (bool, error)
	RunInTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
	MarkImportedTx(tx *gorm.DB, jobID uuid.UUID, imported, skipped, failed int) error
}

type postgresRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreatePreview(ctx context.Context, job *domain.ImportJob, rows []domain.ImportJobRow) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		jobModel := mapJobToModel(job)
		if err := tx.Create(&jobModel).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		models := make([]ImportRowModel, len(rows))
		for i, row := range rows {
			models[i] = mapRowToModel(row)
		}
		return tx.CreateInBatches(&models, 100).Error
	})
}

func (r *postgresRepository) FindJobByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	var model ImportJobModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job := mapJobFromModel(model)
	return &job, nil
}

func (r *postgresRepository) FindRowsByJobID(ctx context.Context, jobID uuid.UUID) ([]domain.ImportJobRow, error) {
	var models []ImportRowModel
	err := r.db.WithContext(ctx).
		Where("import_job_id = ?", jobID).
		Order("row_number ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	rows := make([]domain.ImportJobRow, len(models))
	for i, m := range models {
		rows[i] = mapRowFromModel(m)
	}
	return rows, nil
}

func (r *postgresRepository) ListJobs(ctx context.Context, filter JobListFilter) ([]domain.ImportJob, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, importJobSortColumns, "created_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)

	q := r.db.WithContext(ctx).Model(&ImportJobModel{})
	if filter.Query != "" {
		q = q.Where("filename ILIKE ?", "%"+filter.Query+"%")
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.DateFrom != nil {
		q = q.Where("created_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		q = q.Where("created_at < ?", filter.DateTo.Add(24*time.Hour))
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []ImportJobModel
	err := q.Order(pagination.OrderClause(sortCol, orderDir)).
		Offset(pagination.Offset(page, limit)).
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	jobs := make([]domain.ImportJob, len(models))
	for i, m := range models {
		jobs[i] = mapJobFromModel(m)
	}
	return jobs, total, nil
}

func (r *postgresRepository) MarkImported(ctx context.Context, jobID uuid.UUID, imported, skipped, failed int) error {
	return r.markImported(r.db.WithContext(ctx), jobID, imported, skipped, failed)
}

func (r *postgresRepository) MarkImportedTx(tx *gorm.DB, jobID uuid.UUID, imported, skipped, failed int) error {
	return r.markImported(tx, jobID, imported, skipped, failed)
}

func (r *postgresRepository) markImported(db *gorm.DB, jobID uuid.UUID, imported, skipped, failed int) error {
	now := time.Now().UTC()
	return db.Model(&ImportJobModel{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":             domain.JobStatusImported,
		"imported_questions": imported,
		"skipped_rows":       skipped,
		"failed_rows":        failed,
		"confirmed_at":       now,
	}).Error
}

func (r *postgresRepository) ExistsQuestionText(ctx context.Context, text string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("questions").
		Where("question_text = ?", text).
		Count(&count).Error
	return count > 0, err
}

func (r *postgresRepository) RunInTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func mapJobToModel(job *domain.ImportJob) ImportJobModel {
	return ImportJobModel{
		ID:                job.ID,
		AdminUserID:       job.AdminUserID,
		Filename:          job.Filename,
		Status:            job.Status,
		TotalRows:         job.TotalRows,
		ValidRows:         job.ValidRows,
		InvalidRows:       job.InvalidRows,
		ImportedQuestions: job.ImportedQuestions,
		SkippedRows:       job.SkippedRows,
		FailedRows:        job.FailedRows,
		CreatedAt:         job.CreatedAt,
		ConfirmedAt:       job.ConfirmedAt,
	}
}

func mapJobFromModel(m ImportJobModel) domain.ImportJob {
	return domain.ImportJob{
		ID:                m.ID,
		AdminUserID:       m.AdminUserID,
		Filename:          m.Filename,
		Status:            m.Status,
		TotalRows:         m.TotalRows,
		ValidRows:         m.ValidRows,
		InvalidRows:       m.InvalidRows,
		ImportedQuestions: m.ImportedQuestions,
		SkippedRows:       m.SkippedRows,
		FailedRows:        m.FailedRows,
		CreatedAt:         m.CreatedAt,
		ConfirmedAt:       m.ConfirmedAt,
	}
}

func mapRowToModel(row domain.ImportJobRow) ImportRowModel {
	return ImportRowModel{
		ID:                  row.ID,
		ImportJobID:         row.ImportJobID,
		RowNumber:           row.RowNumber,
		SubjectCode:         row.SubjectCode,
		Tags:                row.Tags,
		QuestionType:        row.QuestionType,
		ContentFormat:       row.ContentFormat,
		QuestionText:        row.QuestionText,
		QuestionImage:       row.QuestionImage,
		QuestionImageURL:    row.QuestionImageURL,
		ChoiceA:             row.ChoiceA,
		ChoiceAImage:        row.ChoiceAImage,
		ChoiceAImageURL:     row.ChoiceAImageURL,
		ChoiceB:             row.ChoiceB,
		ChoiceBImage:        row.ChoiceBImage,
		ChoiceBImageURL:     row.ChoiceBImageURL,
		ChoiceC:             row.ChoiceC,
		ChoiceCImage:        row.ChoiceCImage,
		ChoiceCImageURL:     row.ChoiceCImageURL,
		ChoiceD:             row.ChoiceD,
		ChoiceDImage:        row.ChoiceDImage,
		ChoiceDImageURL:     row.ChoiceDImageURL,
		CorrectChoice:       row.CorrectChoice,
		Explanation:         row.Explanation,
		ExplanationImage:    row.ExplanationImage,
		ExplanationImageURL: row.ExplanationImageURL,
		Difficulty:          row.Difficulty,
		Status:              row.Status,
		Valid:               row.Valid,
		Errors:              StringList(row.Errors),
		Warnings:            StringList(row.Warnings),
		CreatedAt:           row.CreatedAt,
	}
}

func mapRowFromModel(m ImportRowModel) domain.ImportJobRow {
	return domain.ImportJobRow{
		ID:                  m.ID,
		ImportJobID:         m.ImportJobID,
		RowNumber:           m.RowNumber,
		SubjectCode:         m.SubjectCode,
		Tags:                m.Tags,
		QuestionType:        m.QuestionType,
		ContentFormat:       m.ContentFormat,
		QuestionText:        m.QuestionText,
		QuestionImage:       m.QuestionImage,
		QuestionImageURL:    m.QuestionImageURL,
		ChoiceA:             m.ChoiceA,
		ChoiceAImage:        m.ChoiceAImage,
		ChoiceAImageURL:     m.ChoiceAImageURL,
		ChoiceB:             m.ChoiceB,
		ChoiceBImage:        m.ChoiceBImage,
		ChoiceBImageURL:     m.ChoiceBImageURL,
		ChoiceC:             m.ChoiceC,
		ChoiceCImage:        m.ChoiceCImage,
		ChoiceCImageURL:     m.ChoiceCImageURL,
		ChoiceD:             m.ChoiceD,
		ChoiceDImage:        m.ChoiceDImage,
		ChoiceDImageURL:     m.ChoiceDImageURL,
		CorrectChoice:       m.CorrectChoice,
		Explanation:         m.Explanation,
		ExplanationImage:    m.ExplanationImage,
		ExplanationImageURL: m.ExplanationImageURL,
		Difficulty:          m.Difficulty,
		Status:              m.Status,
		Valid:               m.Valid,
		Errors:              []string(m.Errors),
		Warnings:            []string(m.Warnings),
		CreatedAt:           m.CreatedAt,
	}
}
