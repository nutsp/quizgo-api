package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/examset/domain"
)

func (uc *ExamSetUseCase) buildVisibilityScope(ctx context.Context, userID *uuid.UUID) domain.VisibilityScope {
	if userID == nil || uc.entitlements == nil {
		return domain.VisibilityScope{}
	}
	ids, err := uc.entitlements.ListEntitledPrivateExamSetIDs(ctx, *userID)
	if err != nil || len(ids) == 0 {
		return domain.VisibilityScope{}
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return domain.VisibilityScope{EntitledPrivateExamSetIDs: out}
}

func (uc *ExamSetUseCase) FilterOptions(ctx context.Context, userID *uuid.UUID) (*domain.FilterOptionsResponse, error) {
	scope := uc.buildVisibilityScope(ctx, userID)
	scopeKey := cache.FilterOptionsScopeKey(userID, scope)
	key := cache.ExamSetsFilterOptions(scopeKey)

	var cached domain.FilterOptionsResponse
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cached); ok {
		return &cached, nil
	}

	result, err := uc.examSets.AggregateFilterOptions(ctx, scope)
	if err != nil {
		return nil, err
	}

	_ = uc.contentCache.SetJSON(ctx, key, result, cache.TTLExamSetsFilterOptions)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexExamSetsFilterOptions(), key, cache.TTLExamSetsFilterOptions+cache.TTLIndexBuffer)
	return result, nil
}
