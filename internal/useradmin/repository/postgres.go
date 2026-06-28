package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/common/pagination"
	userdomain "virtual-exam-api/internal/user/domain"
	userrepo "virtual-exam-api/internal/user/repository"
)

type UserAdminFilter struct {
	Query  string
	Role   string
	Status string
	Page   int
	Limit  int
	Sort   string
	Order  string
}

var userSortColumns = map[string]string{
	"created_at":    "created_at",
	"updated_at":    "updated_at",
	"email":         "email",
	"display_name":  "display_name",
	"last_login_at": "last_login_at",
	"role":          "role",
	"status":        "status",
}

type UserAdminRepository interface {
	List(ctx context.Context, filter UserAdminFilter) ([]userdomain.User, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role string) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) error
	CountActiveAdmins(ctx context.Context) (int64, error)
	CountActiveAdminsExcept(ctx context.Context, excludeID uuid.UUID) (int64, error)
}

type userAdminRepository struct {
	db *gorm.DB
}

func NewUserAdminRepository(db *gorm.DB) UserAdminRepository {
	return &userAdminRepository{db: db}
}

func (r *userAdminRepository) List(ctx context.Context, filter UserAdminFilter) ([]userdomain.User, int64, error) {
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	sortCol := pagination.ResolveSort(filter.Sort, userSortColumns, "created_at")
	orderDir := pagination.ResolveOrder(filter.Order, true)
	orderClause := pagination.OrderClause(sortCol, orderDir)
	if sortCol == "last_login_at" && orderDir == "DESC" {
		orderClause = "last_login_at DESC NULLS LAST"
	}
	q := r.db.WithContext(ctx).Model(&userrepo.UserModel{})
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("display_name ILIKE ? OR email ILIKE ?", like, like)
	}
	if filter.Role != "" {
		q = q.Where("role = ?", filter.Role)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []userrepo.UserModel
	offset := pagination.Offset(page, limit)
	if err := q.Order(orderClause).Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]userdomain.User, len(models))
	for i := range models {
		items[i] = toDomain(&models[i])
	}
	return items, total, nil
}

func (r *userAdminRepository) FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error) {
	var model userrepo.UserModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user := toDomain(&model)
	return &user, nil
}

func (r *userAdminRepository) UpdateRole(ctx context.Context, id uuid.UUID, role string) error {
	return r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("id = ?", id).
		Update("role", role).Error
}

func (r *userAdminRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *userAdminRepository) UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) error {
	displayName = strings.TrimSpace(displayName)
	return r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("id = ?", id).
		Update("display_name", displayName).Error
}

func (r *userAdminRepository) CountActiveAdmins(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("role = ? AND status = ?", userdomain.RoleAdmin, userdomain.StatusActive).
		Count(&count).Error
	return count, err
}

func (r *userAdminRepository) CountActiveAdminsExcept(ctx context.Context, excludeID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Where("role = ? AND status = ? AND id <> ?", userdomain.RoleAdmin, userdomain.StatusActive, excludeID).
		Count(&count).Error
	return count, err
}

func toDomain(m *userrepo.UserModel) userdomain.User {
	passwordHash := ""
	if m.PasswordHash != nil {
		passwordHash = *m.PasswordHash
	}
	status := m.Status
	if status == "" {
		status = userdomain.StatusActive
	}
	return userdomain.User{
		ID:           m.ID,
		DisplayName:  m.DisplayName,
		Email:        m.Email,
		PasswordHash: passwordHash,
		Role:         m.Role,
		Status:       status,
		LastLoginAt:  m.LastLoginAt,
		AvatarURL:    m.AvatarURL,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
