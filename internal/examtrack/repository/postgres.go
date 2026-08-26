package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/examtrack/domain"
)

type ExamTrackModel struct {
	ID                       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code                     string    `gorm:"uniqueIndex:uq_exam_tracks_code;not null"`
	Name                     string    `gorm:"not null"`
	Description              string
	CoverImageURL            *string
	TotalExamSets            int        `gorm:"default:0"`
	TotalQuestions           int        `gorm:"default:0"`
	TotalAttempts            int        `gorm:"default:0"`
	BlueprintVersion         int        `gorm:"not null;default:1"`
	BlueprintStatus          string     `gorm:"type:varchar(20);not null;default:draft"`
	BlueprintQuestionCount   int        `gorm:"not null;default:0"`
	BlueprintDurationMinutes int        `gorm:"not null;default:0"`
	BlueprintPassingScore    int        `gorm:"not null;default:0"`
	BlueprintEffectiveDate   *time.Time `gorm:"type:date"`
	BlueprintReviewedAt      *time.Time
	BlueprintSourceNote      string                    `gorm:"type:text;not null;default:''"`
	BlueprintSections        []domain.BlueprintSection `gorm:"serializer:json;type:jsonb;not null;default:'[]'"`
	IsActive                 bool                      `gorm:"default:true"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (ExamTrackModel) TableName() string { return "exam_tracks" }

func PrepareBlueprintSectionsMigration(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&ExamTrackModel{}, "BlueprintSections") {
		return nil
	}
	return db.Exec(`UPDATE exam_tracks SET blueprint_sections = '[]'::jsonb WHERE blueprint_sections IS NULL`).Error
}

type Repository interface {
	ListActive(ctx context.Context) ([]domain.ExamTrack, error)
	FindByCode(ctx context.Context, code string) (*domain.ExamTrack, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ExamTrack, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) ListActive(ctx context.Context) ([]domain.ExamTrack, error) {
	var models []ExamTrackModel
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("name ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExamTrack, len(models))
	for i := range models {
		out[i] = toDomain(&models[i])
	}
	return out, nil
}

func (r *postgresRepository) FindByCode(ctx context.Context, code string) (*domain.ExamTrack, error) {
	var model ExamTrackModel
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	track := toDomain(&model)
	return &track, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ExamTrack, error) {
	var model ExamTrackModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	track := toDomain(&model)
	return &track, nil
}

func toDomain(m *ExamTrackModel) domain.ExamTrack {
	blueprint := domain.Blueprint{
		Version:         m.BlueprintVersion,
		Status:          m.BlueprintStatus,
		QuestionCount:   m.BlueprintQuestionCount,
		DurationMinutes: m.BlueprintDurationMinutes,
		PassingScore:    m.BlueprintPassingScore,
		EffectiveDate:   m.BlueprintEffectiveDate,
		ReviewedAt:      m.BlueprintReviewedAt,
		SourceNote:      m.BlueprintSourceNote,
		Sections:        m.BlueprintSections,
	}
	blueprint.Normalize()
	return domain.ExamTrack{
		ID:             m.ID,
		Code:           m.Code,
		Name:           m.Name,
		Description:    m.Description,
		CoverImageURL:  m.CoverImageURL,
		TotalExamSets:  m.TotalExamSets,
		TotalQuestions: m.TotalQuestions,
		TotalAttempts:  m.TotalAttempts,
		Blueprint:      blueprint,
		IsActive:       m.IsActive,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
