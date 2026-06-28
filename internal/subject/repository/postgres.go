package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/common/pagination"
	questionrepo "virtual-exam-api/internal/question/repository"
	"virtual-exam-api/internal/question/domain"
)

type SubjectAdminFilter struct {
	Query string
	Page  int
	Limit int
	Sort  string
	Order string
}

var subjectSortColumns = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"name":       "name",
	"code":       "code",
}

type SubjectAdminRepository interface {
	List(ctx context.Context, filter SubjectAdminFilter) ([]domain.Subject, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Subject, error)
	FindByCode(ctx context.Context, code string) (*domain.Subject, error)
	Create(ctx context.Context, subject *domain.Subject) error
	Update(ctx context.Context, subject *domain.Subject) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountQuestions(ctx context.Context, subjectID uuid.UUID) (int64, error)
}

type subjectAdminRepository struct {
	db *gorm.DB
}

func NewSubjectAdminRepository(db *gorm.DB) SubjectAdminRepository {
	return &subjectAdminRepository{db: db}
}

func (r *subjectAdminRepository) List(ctx context.Context, filter SubjectAdminFilter) ([]domain.Subject, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, subjectSortColumns, "updated_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	q := r.db.WithContext(ctx).Model(&questionrepo.SubjectModel{})
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []questionrepo.SubjectModel
	err := q.Order(pagination.OrderClause(sortCol, orderDir)).Offset(pagination.Offset(page, limit)).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Subject, len(models))
	for i, m := range models {
		out[i] = subjectToDomain(&m)
	}
	return out, total, nil
}

func (r *subjectAdminRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Subject, error) {
	var model questionrepo.SubjectModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := subjectToDomain(&model)
	return &s, nil
}

func (r *subjectAdminRepository) FindByCode(ctx context.Context, code string) (*domain.Subject, error) {
	var model questionrepo.SubjectModel
	err := r.db.WithContext(ctx).Where("code = ?", strings.ToLower(code)).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := subjectToDomain(&model)
	return &s, nil
}

func (r *subjectAdminRepository) Create(ctx context.Context, subject *domain.Subject) error {
	if subject.ID == uuid.Nil {
		subject.ID = uuid.New()
	}
	now := time.Now().UTC()
	subject.CreatedAt = now
	subject.UpdatedAt = now
	model := questionrepo.SubjectModel{
		ID:          subject.ID,
		Code:        strings.ToLower(subject.Code),
		Name:        subject.Name,
		Description: subject.Description,
		CreatedAt:   subject.CreatedAt,
		UpdatedAt:   subject.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *subjectAdminRepository) Update(ctx context.Context, subject *domain.Subject) error {
	subject.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Model(&questionrepo.SubjectModel{}).Where("id = ?", subject.ID).Updates(map[string]any{
		"code":        strings.ToLower(subject.Code),
		"name":        subject.Name,
		"description": subject.Description,
		"updated_at":  subject.UpdatedAt,
	}).Error
}

func (r *subjectAdminRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var qCount int64
	if err := r.db.WithContext(ctx).Model(&questionrepo.QuestionModel{}).Where("subject_id = ?", id).Count(&qCount).Error; err != nil {
		return err
	}
	if qCount > 0 {
		return gorm.ErrInvalidData
	}
	return r.db.WithContext(ctx).Delete(&questionrepo.SubjectModel{}, "id = ?", id).Error
}

func (r *subjectAdminRepository) CountQuestions(ctx context.Context, subjectID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&questionrepo.QuestionModel{}).Where("subject_id = ?", subjectID).Count(&count).Error
	return count, err
}

func subjectToDomain(m *questionrepo.SubjectModel) domain.Subject {
	return domain.Subject{
		ID:          m.ID,
		Code:        m.Code,
		Name:        m.Name,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func CountAllSubjects(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&questionrepo.SubjectModel{}).Count(&count).Error
	return count, err
}

func IsValidSubjectCode(code string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
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
