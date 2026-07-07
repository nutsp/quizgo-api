package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SystemSettingModel struct {
	Key         string `gorm:"primaryKey;type:text"`
	Value       []byte `gorm:"type:jsonb;not null"`
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SystemSettingModel) TableName() string { return "system_settings" }

type SettingRecord struct {
	Key       string
	Value     []byte
	UpdatedAt time.Time
}

type Repository interface {
	Get(ctx context.Context, key string) (*SettingRecord, error)
	Upsert(ctx context.Context, key string, value any, description string) (*SettingRecord, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Get(ctx context.Context, key string) (*SettingRecord, error) {
	var model SystemSettingModel
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &SettingRecord{Key: model.Key, Value: model.Value, UpdatedAt: model.UpdatedAt}, nil
}

func (r *postgresRepository) Upsert(ctx context.Context, key string, value any, description string) (*SettingRecord, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	desc := description
	now := time.Now().UTC()
	model := SystemSettingModel{
		Key:         key,
		Value:       raw,
		Description: &desc,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":       raw,
			"description": desc,
			"updated_at":  now,
		}),
	}).Create(&model).Error
	if err != nil {
		return nil, err
	}
	record, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return record, nil
}
