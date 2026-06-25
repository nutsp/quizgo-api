package repository

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/examset/domain"
)

type ExamSetModel struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamTrackID     uuid.UUID `gorm:"type:uuid;not null;index"`
	Code            string    `gorm:"uniqueIndex:uq_exam_sets_code;not null"`
	Title           string    `gorm:"not null"`
	Description     string
	DurationMinutes int    `gorm:"not null"`
	TotalQuestions  int    `gorm:"not null"`
	PassingScore    int    `gorm:"not null"`
	Difficulty      string `gorm:"not null"`
	AccessType      string `gorm:"not null"`
	Mode            string `gorm:"not null"`
	IsOfficial      bool   `gorm:"default:false"`
	IsActive        bool   `gorm:"default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time

	ExamTrack ExamTrackJoin `gorm:"foreignKey:ExamTrackID;references:ID"`
}

type ExamTrackJoin struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code string
	Name string
}

func (ExamTrackJoin) TableName() string { return "exam_tracks" }

func (ExamSetModel) TableName() string { return "exam_sets" }

type Repository interface {
	List(ctx context.Context, filter domain.ListFilter) (*domain.PaginatedResult, error)
	FindByCode(ctx context.Context, code string) (*domain.ExamSet, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ExamSet, error)
	ListPopular(ctx context.Context, limit int) ([]domain.ExamSet, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) List(ctx context.Context, filter domain.ListFilter) (*domain.PaginatedResult, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := r.db.WithContext(ctx).Model(&ExamSetModel{}).Preload("ExamTrack")
	if filter.OnlyActive {
		q = q.Where("exam_sets.is_active = ?", true)
	}
	if filter.TrackID != uuid.Nil {
		q = q.Where("exam_track_id = ?", filter.TrackID)
	}
	if filter.AccessType != "" {
		q = q.Where("access_type = ?", filter.AccessType)
	}
	if filter.Difficulty != "" {
		q = q.Where("difficulty = ?", filter.Difficulty)
	}
	if filter.Mode != "" {
		q = q.Where("mode = ?", filter.Mode)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("title ILIKE ? OR description ILIKE ? OR code ILIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	var models []ExamSetModel
	offset := (page - 1) * limit
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}

	items := make([]domain.ExamSetSummary, len(models))
	for i := range models {
		set := toDomain(&models[i])
		items[i] = set.ToSummary()
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	return &domain.PaginatedResult{
		Items:      items,
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

func (r *postgresRepository) FindByCode(ctx context.Context, code string) (*domain.ExamSet, error) {
	var model ExamSetModel
	err := r.db.WithContext(ctx).Preload("ExamTrack").Where("code = ?", code).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := toDomain(&model)
	return &set, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ExamSet, error) {
	var model ExamSetModel
	err := r.db.WithContext(ctx).Preload("ExamTrack").Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := toDomain(&model)
	return &set, nil
}

func (r *postgresRepository) ListPopular(ctx context.Context, limit int) ([]domain.ExamSet, error) {
	if limit < 1 {
		limit = 4
	}
	var models []ExamSetModel
	err := r.db.WithContext(ctx).
		Preload("ExamTrack").
		Where("is_active = ?", true).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExamSet, len(models))
	for i := range models {
		out[i] = toDomain(&models[i])
	}
	return out, nil
}

func toDomain(m *ExamSetModel) domain.ExamSet {
	set := domain.ExamSet{
		ID:              m.ID,
		ExamTrackID:     m.ExamTrackID,
		Code:            m.Code,
		Title:           m.Title,
		Description:     m.Description,
		DurationMinutes: m.DurationMinutes,
		TotalQuestions:  m.TotalQuestions,
		PassingScore:    m.PassingScore,
		Difficulty:      m.Difficulty,
		AccessType:      m.AccessType,
		Mode:            m.Mode,
		IsOfficial:      m.IsOfficial,
		IsActive:        m.IsActive,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if m.ExamTrack.Code != "" {
		set.ExamTrack = &domain.ExamTrackRef{Code: m.ExamTrack.Code, Name: m.ExamTrack.Name}
	}
	return set
}
