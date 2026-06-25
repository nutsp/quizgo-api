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
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"not null;default:user"`
	AvatarURL    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string { return "users" }

type Repository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
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

func toModel(u *domain.User) UserModel {
	return UserModel{
		ID:           u.ID,
		DisplayName:  u.DisplayName,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		AvatarURL:    u.AvatarURL,
	}
}

func toDomain(m *UserModel) domain.User {
	return domain.User{
		ID:           m.ID,
		DisplayName:  m.DisplayName,
		Email:        m.Email,
		PasswordHash: m.PasswordHash,
		Role:         m.Role,
		AvatarURL:    m.AvatarURL,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
