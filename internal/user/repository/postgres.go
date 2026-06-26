package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/user/domain"
)

type UserModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	DisplayName  string    `gorm:"not null"`
	Email        string    `gorm:"uniqueIndex:uq_users_email;not null"`
	PasswordHash *string
	Role         string    `gorm:"not null;default:user"`
	AvatarURL    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string { return "users" }

type Repository interface {
	Create(ctx context.Context, user *domain.User) error
	CreateOAuthUser(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) error
	UpdateDisplayNameIfEmpty(ctx context.Context, id uuid.UUID, displayName string) error
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, user *domain.User) error {
	model := toModel(user)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}
	*user = toDomain(&model)
	return nil
}

func (r *postgresRepository) CreateOAuthUser(ctx context.Context, user *domain.User) error {
	model := toModel(user)
	model.PasswordHash = nil
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}
	*user = toDomain(&model)
	return nil
}

func (r *postgresRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user := toDomain(&model)
	return &user, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var model UserModel
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

func (r *postgresRepository) UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) error {
	return r.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", id).
		Update("display_name", displayName).Error
}

func (r *postgresRepository) UpdateDisplayNameIfEmpty(ctx context.Context, id uuid.UUID, displayName string) error {
	return r.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ? AND (display_name IS NULL OR display_name = '')", id).
		Update("display_name", displayName).Error
}

func toModel(u *domain.User) UserModel {
	var passwordHash *string
	if u.PasswordHash != "" {
		passwordHash = &u.PasswordHash
	}
	return UserModel{
		ID:           u.ID,
		DisplayName:  u.DisplayName,
		Email:        u.Email,
		PasswordHash: passwordHash,
		Role:         u.Role,
		AvatarURL:    u.AvatarURL,
	}
}

func toDomain(m *UserModel) domain.User {
	passwordHash := ""
	if m.PasswordHash != nil {
		passwordHash = *m.PasswordHash
	}
	return domain.User{
		ID:           m.ID,
		DisplayName:  m.DisplayName,
		Email:        m.Email,
		PasswordHash: passwordHash,
		Role:         m.Role,
		AvatarURL:    m.AvatarURL,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
