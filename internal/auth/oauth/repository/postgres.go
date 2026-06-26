package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	oauthdomain "virtual-exam-api/internal/auth/oauth/domain"
)

type OAuthAccountModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID `gorm:"type:uuid;not null;index:idx_user_oauth_accounts_user_id"`
	Provider          string    `gorm:"not null;uniqueIndex:uq_user_oauth_provider_user,priority:1"`
	ProviderUserID    string    `gorm:"not null;uniqueIndex:uq_user_oauth_provider_user,priority:2"`
	ProviderEmail     *string
	ProviderName      *string
	ProviderAvatarURL *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (OAuthAccountModel) TableName() string { return "user_oauth_accounts" }

type Repository interface {
	FindByProviderAndUserID(ctx context.Context, provider, providerUserID string) (*oauthdomain.OAuthAccount, error)
	Create(ctx context.Context, account *oauthdomain.OAuthAccount) error
	UpdateProviderProfile(ctx context.Context, account *oauthdomain.OAuthAccount) error
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) FindByProviderAndUserID(ctx context.Context, provider, providerUserID string) (*oauthdomain.OAuthAccount, error) {
	var model OAuthAccountModel
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	account := toDomain(&model)
	return &account, nil
}

func (r *postgresRepository) Create(ctx context.Context, account *oauthdomain.OAuthAccount) error {
	model := toModel(account)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}
	*account = toDomain(&model)
	return nil
}

func (r *postgresRepository) UpdateProviderProfile(ctx context.Context, account *oauthdomain.OAuthAccount) error {
	return r.db.WithContext(ctx).Model(&OAuthAccountModel{}).
		Where("id = ?", account.ID).
		Updates(map[string]any{
			"provider_email":      account.ProviderEmail,
			"provider_name":       account.ProviderName,
			"provider_avatar_url": account.ProviderAvatarURL,
			"updated_at":          time.Now().UTC(),
		}).Error
}

func toModel(a *oauthdomain.OAuthAccount) OAuthAccountModel {
	return OAuthAccountModel{
		ID:                a.ID,
		UserID:            a.UserID,
		Provider:          a.Provider,
		ProviderUserID:    a.ProviderUserID,
		ProviderEmail:     a.ProviderEmail,
		ProviderName:      a.ProviderName,
		ProviderAvatarURL: a.ProviderAvatarURL,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
	}
}

func toDomain(m *OAuthAccountModel) oauthdomain.OAuthAccount {
	return oauthdomain.OAuthAccount{
		ID:                m.ID,
		UserID:            m.UserID,
		Provider:          m.Provider,
		ProviderUserID:    m.ProviderUserID,
		ProviderEmail:     m.ProviderEmail,
		ProviderName:      m.ProviderName,
		ProviderAvatarURL: m.ProviderAvatarURL,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}
