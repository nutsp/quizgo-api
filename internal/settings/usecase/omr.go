package usecase

import (
	"context"
	"encoding/json"
	"time"

	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/settings/domain"
	"virtual-exam-api/internal/settings/repository"
)

type UseCase struct {
	settings repository.Repository
	cache    cache.CacheService
}

func NewUseCase(settings repository.Repository, cacheService cache.CacheService) *UseCase {
	if cacheService == nil {
		cacheService = cache.Noop()
	}
	return &UseCase{settings: settings, cache: cacheService}
}

func (uc *UseCase) GetOMR(ctx context.Context) (*domain.OMRAnswerSheetSettings, error) {
	key := cache.SettingsOMRAnswerSheet()
	var cached domain.OMRAnswerSheetSettings
	if ok, _ := uc.cache.GetJSON(ctx, key, &cached); ok {
		return &cached, nil
	}

	settings := domain.DefaultOMRAnswerSheetSettings()
	record, err := uc.settings.Get(ctx, domain.OMRAnswerSheetKey)
	if err != nil {
		return nil, err
	}
	if record != nil {
		var stored domain.OMRAnswerSheetSettings
		if err := json.Unmarshal(record.Value, &stored); err == nil && stored.Validate() == nil {
			stored.UpdatedAt = &record.UpdatedAt
			settings = stored
		}
	}

	_ = uc.cache.SetJSON(ctx, key, settings, cache.TTLSettingsOMR)
	return &settings, nil
}

func (uc *UseCase) UpdateOMR(ctx context.Context, input domain.OMRAnswerSheetSettings) (*domain.OMRAnswerSheetSettings, error) {
	if err := input.Validate(); err != nil {
		return nil, apperrors.ValidationError("ตั้งค่ากระดาษคำตอบ OMR ไม่ถูกต้อง")
	}

	record, err := uc.settings.Upsert(ctx, domain.OMRAnswerSheetKey, input, "Global OMR answer sheet settings")
	if err != nil {
		return nil, err
	}
	input.UpdatedAt = updatedAtPtr(record)
	_ = uc.cache.Delete(ctx, cache.SettingsOMRAnswerSheet())
	return &input, nil
}

func updatedAtPtr(record *repository.SettingRecord) *time.Time {
	if record == nil {
		return nil
	}
	t := record.UpdatedAt
	return &t
}
