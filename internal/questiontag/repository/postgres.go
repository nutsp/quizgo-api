package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/questiontag/domain"
)

type QuestionTagModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"type:varchar(100);not null"`
	Code        string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	Description string    `gorm:"type:text"`
	Color       string    `gorm:"type:varchar(20)"`
	IsActive    bool      `gorm:"not null;default:true"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (QuestionTagModel) TableName() string { return "question_tags" }

type QuestionTagMappingModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	QuestionID uuid.UUID `gorm:"type:uuid;not null;index"`
	TagID      uuid.UUID `gorm:"type:uuid;not null;index"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (QuestionTagMappingModel) TableName() string { return "question_tag_mappings" }

type TagAdminFilter struct {
	Query    string
	IsActive *bool
	Page     int
	Limit    int
	Sort     string
	Order    string
}

var tagSortColumns = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"name":       "name",
	"code":       "code",
}

type TagAdminRepository interface {
	List(ctx context.Context, filter TagAdminFilter) ([]domain.QuestionTag, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.QuestionTag, error)
	FindByCode(ctx context.Context, code string) (*domain.QuestionTag, error)
	FindActiveByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.QuestionTag, error)
	FindActiveByCodes(ctx context.Context, codes []string) ([]domain.QuestionTag, error)
	Create(ctx context.Context, tag *domain.QuestionTag) error
	Update(ctx context.Context, tag *domain.QuestionTag) error
	Delete(ctx context.Context, id uuid.UUID) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	CountQuestions(ctx context.Context, tagID uuid.UUID) (int64, error)
	ListActive(ctx context.Context) ([]domain.QuestionTag, error)
	LoadTagsForQuestions(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID][]domain.TagRef, error)
	ReplaceQuestionTagMappingsTx(tx *gorm.DB, questionID uuid.UUID, tagIDs []uuid.UUID) error
}

type tagAdminRepository struct {
	db *gorm.DB
}

func NewTagAdminRepository(db *gorm.DB) TagAdminRepository {
	return &tagAdminRepository{db: db}
}

func (r *tagAdminRepository) List(ctx context.Context, filter TagAdminFilter) ([]domain.QuestionTag, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, tagSortColumns, "updated_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	q := r.db.WithContext(ctx).Model(&QuestionTagModel{})
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
	var models []QuestionTagModel
	err := q.Order(pagination.OrderClause(sortCol, orderDir)).Offset(pagination.Offset(page, limit)).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.QuestionTag, len(models))
	for i, m := range models {
		out[i] = tagToDomain(&m)
	}
	return out, total, nil
}

func (r *tagAdminRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.QuestionTag, error) {
	var model QuestionTagModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t := tagToDomain(&model)
	return &t, nil
}

func (r *tagAdminRepository) FindByCode(ctx context.Context, code string) (*domain.QuestionTag, error) {
	var model QuestionTagModel
	err := r.db.WithContext(ctx).Where("code = ?", strings.ToLower(strings.TrimSpace(code))).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t := tagToDomain(&model)
	return &t, nil
}

func (r *tagAdminRepository) FindActiveByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.QuestionTag, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var models []QuestionTagModel
	err := r.db.WithContext(ctx).Where("id IN ? AND is_active = ?", ids, true).Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.QuestionTag, len(models))
	for i, m := range models {
		out[i] = tagToDomain(&m)
	}
	return out, nil
}

func (r *tagAdminRepository) FindActiveByCodes(ctx context.Context, codes []string) ([]domain.QuestionTag, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(codes))
	for _, c := range codes {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(c)))
	}
	var models []QuestionTagModel
	err := r.db.WithContext(ctx).Where("code IN ? AND is_active = ?", normalized, true).Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.QuestionTag, len(models))
	for i, m := range models {
		out[i] = tagToDomain(&m)
	}
	return out, nil
}

func (r *tagAdminRepository) Create(ctx context.Context, tag *domain.QuestionTag) error {
	if tag.ID == uuid.Nil {
		tag.ID = uuid.New()
	}
	now := time.Now().UTC()
	tag.CreatedAt = now
	tag.UpdatedAt = now
	model := QuestionTagModel{
		ID:          tag.ID,
		Name:        tag.Name,
		Code:        strings.ToLower(tag.Code),
		Description: tag.Description,
		Color:       tag.Color,
		IsActive:    tag.IsActive,
		CreatedAt:   tag.CreatedAt,
		UpdatedAt:   tag.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *tagAdminRepository) Update(ctx context.Context, tag *domain.QuestionTag) error {
	tag.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Model(&QuestionTagModel{}).Where("id = ?", tag.ID).Updates(map[string]any{
		"name":        tag.Name,
		"code":        strings.ToLower(tag.Code),
		"description": tag.Description,
		"color":       tag.Color,
		"is_active":   tag.IsActive,
		"updated_at":  tag.UpdatedAt,
	}).Error
}

func (r *tagAdminRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&QuestionTagMappingModel{}).Where("tag_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return gorm.ErrInvalidData
	}
	return r.db.WithContext(ctx).Delete(&QuestionTagModel{}, "id = ?", id).Error
}

func (r *tagAdminRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&QuestionTagModel{}).Where("id = ?", id).Updates(map[string]any{
		"is_active":  false,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *tagAdminRepository) CountQuestions(ctx context.Context, tagID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&QuestionTagMappingModel{}).Where("tag_id = ?", tagID).Count(&count).Error
	return count, err
}

func (r *tagAdminRepository) ListActive(ctx context.Context) ([]domain.QuestionTag, error) {
	var models []QuestionTagModel
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("name ASC").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.QuestionTag, len(models))
	for i, m := range models {
		out[i] = tagToDomain(&m)
	}
	return out, nil
}

func (r *tagAdminRepository) LoadTagsForQuestions(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID][]domain.TagRef, error) {
	result := make(map[uuid.UUID][]domain.TagRef)
	if len(questionIDs) == 0 {
		return result, nil
	}
	type row struct {
		QuestionID uuid.UUID
		TagID      uuid.UUID
		Name       string
		Code       string
		Color      string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("question_tag_mappings m").
		Select("m.question_id, t.id as tag_id, t.name, t.code, t.color").
		Joins("JOIN question_tags t ON t.id = m.tag_id").
		Where("m.question_id IN ?", questionIDs).
		Order("t.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.QuestionID] = append(result[row.QuestionID], domain.TagRef{
			ID:    row.TagID,
			Name:  row.Name,
			Code:  row.Code,
			Color: row.Color,
		})
	}
	return result, nil
}

func (r *tagAdminRepository) ReplaceQuestionTagMappingsTx(tx *gorm.DB, questionID uuid.UUID, tagIDs []uuid.UUID) error {
	if err := tx.Where("question_id = ?", questionID).Delete(&QuestionTagMappingModel{}).Error; err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, tagID := range tagIDs {
		m := QuestionTagMappingModel{
			ID:         uuid.New(),
			QuestionID: questionID,
			TagID:      tagID,
			CreatedAt:  now,
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
	}
	return nil
}

func tagToDomain(m *QuestionTagModel) domain.QuestionTag {
	return domain.QuestionTag{
		ID:          m.ID,
		Name:        m.Name,
		Code:        m.Code,
		Description: m.Description,
		Color:       m.Color,
		IsActive:    m.IsActive,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func IsValidTagCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 100 {
		return false
	}
	for _, ch := range code {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			continue
		}
		return false
	}
	return true
}

func adminPagination(page, limit int) (int, int) {
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
