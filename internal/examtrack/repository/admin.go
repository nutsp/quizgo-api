package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/examtrack/domain"
)

var codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

type AdminFilter struct {
	Query    string
	IsActive *bool
	Page     int
	Limit    int
	Sort     string
	Order    string
}

var trackSortColumns = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"name":       "name",
	"code":       "code",
}

type AdminRepository interface {
	List(ctx context.Context, filter AdminFilter) ([]domain.ExamTrack, int64, error)
	Create(ctx context.Context, track *domain.ExamTrack) error
	Update(ctx context.Context, track *domain.ExamTrack) error
	Delete(ctx context.Context, id uuid.UUID) (deactivated bool, err error)
	CountExamSets(ctx context.Context, trackID uuid.UUID) (int64, error)
	RefreshCounters(ctx context.Context, trackID uuid.UUID) error
}

type adminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) List(ctx context.Context, filter AdminFilter) ([]domain.ExamTrack, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, trackSortColumns, "updated_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	q := r.db.WithContext(ctx).Model(&ExamTrackModel{})
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []ExamTrackModel
	err := q.Order(pagination.OrderClause(sortCol, orderDir)).Offset(pagination.Offset(page, limit)).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.ExamTrack, len(models))
	for i := range models {
		out[i] = toDomain(&models[i])
	}
	return out, total, nil
}

func (r *adminRepository) Create(ctx context.Context, track *domain.ExamTrack) error {
	if track.ID == uuid.Nil {
		track.ID = uuid.New()
	}
	now := time.Now().UTC()
	track.CreatedAt = now
	track.UpdatedAt = now
	model := ExamTrackModel{
		ID:            track.ID,
		Code:          strings.ToLower(track.Code),
		Name:          track.Name,
		Description:   track.Description,
		CoverImageURL: track.CoverImageURL,
		IsActive:      track.IsActive,
		CreatedAt:     track.CreatedAt,
		UpdatedAt:     track.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *adminRepository) Update(ctx context.Context, track *domain.ExamTrack) error {
	track.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Model(&ExamTrackModel{}).Where("id = ?", track.ID).Updates(map[string]any{
		"name":            track.Name,
		"code":            strings.ToLower(track.Code),
		"description":     track.Description,
		"cover_image_url": track.CoverImageURL,
		"is_active":       track.IsActive,
		"updated_at":      track.UpdatedAt,
	}).Error
}

func (r *adminRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	var setCount int64
	if err := r.db.WithContext(ctx).Table("exam_sets").Where("exam_track_id = ?", id).Count(&setCount).Error; err != nil {
		return false, err
	}
	if setCount > 0 {
		err := r.db.WithContext(ctx).Model(&ExamTrackModel{}).Where("id = ?", id).Updates(map[string]any{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		}).Error
		return true, err
	}
	err := r.db.WithContext(ctx).Delete(&ExamTrackModel{}, "id = ?", id).Error
	return false, err
}

func (r *adminRepository) CountExamSets(ctx context.Context, trackID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("exam_sets").Where("exam_track_id = ?", trackID).Count(&count).Error
	return count, err
}

func (r *adminRepository) RefreshCounters(ctx context.Context, trackID uuid.UUID) error {
	var setCount int64
	var totalQuestions int64
	if err := r.db.WithContext(ctx).Table("exam_sets").Where("exam_track_id = ?", trackID).Count(&setCount).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Table("exam_sets").Where("exam_track_id = ?", trackID).
		Select("COALESCE(SUM(total_questions), 0)").Scan(&totalQuestions).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&ExamTrackModel{}).Where("id = ?", trackID).Updates(map[string]any{
		"total_exam_sets": setCount,
		"total_questions": totalQuestions,
		"updated_at":      time.Now().UTC(),
	}).Error
}

func IsValidTrackCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return false
	}
	return codePattern.MatchString(code)
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

func FindTrackByIDAdmin(ctx context.Context, db *gorm.DB, id uuid.UUID) (*domain.ExamTrack, error) {
	var model ExamTrackModel
	err := db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	track := toDomain(&model)
	return &track, nil
}

func FindTrackByCodeAdmin(ctx context.Context, db *gorm.DB, code string) (*domain.ExamTrack, error) {
	var model ExamTrackModel
	err := db.WithContext(ctx).Where("code = ?", strings.ToLower(code)).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	track := toDomain(&model)
	return &track, nil
}
