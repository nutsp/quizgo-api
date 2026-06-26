package repository

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/examset/domain"
)

type AdminFilter struct {
	Query       string
	TrackID     uuid.UUID
	AccessType  string
	Difficulty  string
	Mode        string
	IsActive    *bool
	Page        int
	Limit       int
}

type AdminRepository interface {
	List(ctx context.Context, filter AdminFilter) (*domain.PaginatedResult, error)
	Create(ctx context.Context, set *domain.ExamSet) error
	Update(ctx context.Context, set *domain.ExamSet) error
	Delete(ctx context.Context, id uuid.UUID) (deactivated bool, err error)
	UpdateTotalQuestions(ctx context.Context, examSetID uuid.UUID, count int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, isActive bool) error
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) List(ctx context.Context, filter AdminFilter) (*domain.PaginatedResult, error) {
	page, limit := normalizePagination(filter.Page, filter.Limit)
	q := r.db.WithContext(ctx).Model(&ExamSetModel{}).Preload("ExamTrack")
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("title ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
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
	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var models []ExamSetModel
	err := q.Order("updated_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&models).Error
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

func (r *adminRepository) Create(ctx context.Context, set *domain.ExamSet) error {
	if set.ID == uuid.Nil {
		set.ID = uuid.New()
	}
	now := time.Now().UTC()
	set.CreatedAt = now
	set.UpdatedAt = now
	if set.Currency == "" {
		set.Currency = "THB"
	}
	model := ExamSetModel{
		ID:              set.ID,
		ExamTrackID:     set.ExamTrackID,
		Code:            strings.ToLower(set.Code),
		Title:           set.Title,
		Description:     set.Description,
		CoverImageURL:   set.CoverImageURL,
		DurationMinutes: set.DurationMinutes,
		TotalQuestions:  set.TotalQuestions,
		PassingScore:    set.PassingScore,
		Difficulty:      set.Difficulty,
		AccessType:      set.AccessType,
		PriceAmount:     set.PriceAmount,
		Currency:        set.Currency,
		SalePriceAmount: set.SalePriceAmount,
		Mode:            set.Mode,
		IsOfficial:      set.IsOfficial,
		IsFeatured:      set.IsFeatured,
		IsActive:        set.IsActive,
		Status:          domain.StatusDraft,
		CreatedAt:       set.CreatedAt,
		UpdatedAt:       set.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *adminRepository) Update(ctx context.Context, set *domain.ExamSet) error {
	set.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Model(&ExamSetModel{}).Where("id = ?", set.ID).Updates(map[string]any{
		"exam_track_id":     set.ExamTrackID,
		"code":              strings.ToLower(set.Code),
		"title":             set.Title,
		"description":       set.Description,
		"cover_image_url":   set.CoverImageURL,
		"duration_minutes":  set.DurationMinutes,
		"total_questions":   set.TotalQuestions,
		"passing_score":     set.PassingScore,
		"difficulty":        set.Difficulty,
		"access_type":       set.AccessType,
		"price_amount":      set.PriceAmount,
		"currency":          set.Currency,
		"sale_price_amount": set.SalePriceAmount,
		"mode":              set.Mode,
		"is_official":       set.IsOfficial,
		"is_featured":       set.IsFeatured,
		"is_active":         set.IsActive,
		"updated_at":        set.UpdatedAt,
	}).Error
}

func (r *adminRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	var attemptCount int64
	if err := r.db.WithContext(ctx).Table("exam_attempts").Where("exam_set_id = ?", id).Count(&attemptCount).Error; err != nil {
		return false, err
	}
	if attemptCount > 0 {
		err := r.db.WithContext(ctx).Model(&ExamSetModel{}).Where("id = ?", id).Updates(map[string]any{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		}).Error
		return true, err
	}
	err := r.db.WithContext(ctx).Delete(&ExamSetModel{}, "id = ?", id).Error
	return false, err
}

func (r *adminRepository) UpdateTotalQuestions(ctx context.Context, examSetID uuid.UUID, count int) error {
	return r.db.WithContext(ctx).Model(&ExamSetModel{}).Where("id = ?", examSetID).Updates(map[string]any{
		"total_questions": count,
		"updated_at":      time.Now().UTC(),
	}).Error
}

func (r *adminRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, isActive bool) error {
	return r.db.WithContext(ctx).Model(&ExamSetModel{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"is_active":  isActive,
		"updated_at": time.Now().UTC(),
	}).Error
}

func normalizePagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func IsValidSetCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 80 {
		return false
	}
	for _, ch := range code {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func FindSetByIDAdmin(ctx context.Context, db *gorm.DB, id uuid.UUID) (*domain.ExamSet, error) {
	var model ExamSetModel
	err := db.WithContext(ctx).Preload("ExamTrack").Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := toDomain(&model)
	return &set, nil
}

func FindSetByCodeAdmin(ctx context.Context, db *gorm.DB, code string) (*domain.ExamSet, error) {
	var model ExamSetModel
	err := db.WithContext(ctx).Preload("ExamTrack").Where("code = ?", strings.ToLower(code)).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := toDomain(&model)
	return &set, nil
}

func CountActiveSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&ExamSetModel{}).
		Where("status = ? AND is_active = ?", domain.StatusPublished, true).
		Count(&count).Error
	return count, err
}

func CountPremiumSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&ExamSetModel{}).Where("access_type = ?", domain.AccessPremium).Count(&count).Error
	return count, err
}

func CountFreeSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&ExamSetModel{}).Where("access_type = ?", domain.AccessFree).Count(&count).Error
	return count, err
}

func ListLatestSets(ctx context.Context, db *gorm.DB, limit int) ([]domain.ExamSet, error) {
	if limit < 1 {
		limit = 5
	}
	var models []ExamSetModel
	err := db.WithContext(ctx).Preload("ExamTrack").Order("created_at DESC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExamSet, len(models))
	for i := range models {
		out[i] = toDomain(&models[i])
	}
	return out, nil
}
