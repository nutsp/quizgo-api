package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/questiontag/domain"
	tagrepo "virtual-exam-api/internal/questiontag/repository"
)

type TagUseCase struct {
	tags tagrepo.TagAdminRepository
}

func NewTagUseCase(tags tagrepo.TagAdminRepository) *TagUseCase {
	return &TagUseCase{tags: tags}
}

type TagInput struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Color       string `json:"color"`
	IsActive    *bool  `json:"is_active"`
}

type TagResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Code          string `json:"code"`
	Description   string `json:"description,omitempty"`
	Color         string `json:"color,omitempty"`
	IsActive      bool   `json:"is_active"`
	QuestionCount int64  `json:"question_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type TagListResponse = pagination.PaginatedList[TagResponse]

func (uc *TagUseCase) List(ctx context.Context, filter tagrepo.TagAdminFilter) (*TagListResponse, error) {
	items, total, err := uc.tags.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]TagResponse, len(items))
	for i, t := range items {
		count, _ := uc.tags.CountQuestions(ctx, t.ID)
		resp[i] = toTagResponse(t, count)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func (uc *TagUseCase) Get(ctx context.Context, id uuid.UUID) (*TagResponse, error) {
	t, err := uc.tags.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apperrors.ErrNotFound
	}
	count, _ := uc.tags.CountQuestions(ctx, t.ID)
	resp := toTagResponse(*t, count)
	return &resp, nil
}

func (uc *TagUseCase) Create(ctx context.Context, input TagInput) (*TagResponse, error) {
	if input.Name == "" {
		return nil, apperrors.ValidationError("กรุณาระบุชื่อกลุ่มคำถาม")
	}
	if input.Code == "" {
		return nil, apperrors.ValidationError("กรุณาระบุรหัสกลุ่มคำถาม")
	}
	if !tagrepo.IsValidTagCode(input.Code) {
		return nil, apperrors.ErrInvalidInput
	}
	existing, err := uc.tags.FindByCode(ctx, input.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.ErrCodeTaken
	}
	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	tag := domain.QuestionTag{
		Name:        input.Name,
		Code:        input.Code,
		Description: input.Description,
		Color:       input.Color,
		IsActive:    isActive,
	}
	if err := uc.tags.Create(ctx, &tag); err != nil {
		return nil, err
	}
	resp := toTagResponse(tag, 0)
	return &resp, nil
}

func (uc *TagUseCase) Update(ctx context.Context, id uuid.UUID, input TagInput) (*TagResponse, error) {
	t, err := uc.tags.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apperrors.ErrNotFound
	}
	if input.Name == "" {
		return nil, apperrors.ValidationError("กรุณาระบุชื่อกลุ่มคำถาม")
	}
	if input.Code == "" {
		return nil, apperrors.ValidationError("กรุณาระบุรหัสกลุ่มคำถาม")
	}
	if !tagrepo.IsValidTagCode(input.Code) {
		return nil, apperrors.ErrInvalidInput
	}
	if input.Code != t.Code {
		existing, err := uc.tags.FindByCode(ctx, input.Code)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, apperrors.ErrCodeTaken
		}
	}
	t.Name = input.Name
	t.Code = input.Code
	t.Description = input.Description
	t.Color = input.Color
	if input.IsActive != nil {
		t.IsActive = *input.IsActive
	}
	if err := uc.tags.Update(ctx, t); err != nil {
		return nil, err
	}
	count, _ := uc.tags.CountQuestions(ctx, t.ID)
	resp := toTagResponse(*t, count)
	return &resp, nil
}

func (uc *TagUseCase) Delete(ctx context.Context, id uuid.UUID) (*TagResponse, error) {
	t, err := uc.tags.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apperrors.ErrNotFound
	}
	count, err := uc.tags.CountQuestions(ctx, id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if err := uc.tags.Deactivate(ctx, id); err != nil {
			return nil, err
		}
		t.IsActive = false
		resp := toTagResponse(*t, count)
		return &resp, nil
	}
	if err := uc.tags.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			if deactivateErr := uc.tags.Deactivate(ctx, id); deactivateErr != nil {
				return nil, deactivateErr
			}
			t.IsActive = false
			resp := toTagResponse(*t, count)
			return &resp, nil
		}
		return nil, err
	}
	return nil, nil
}

func (uc *TagUseCase) ValidateTagIDs(ctx context.Context, tagIDs []uuid.UUID) error {
	if len(tagIDs) == 0 {
		return nil
	}
	tags, err := uc.tags.FindActiveByIDs(ctx, tagIDs)
	if err != nil {
		return err
	}
	if len(tags) != len(tagIDs) {
		return apperrors.ErrTagNotFound
	}
	return nil
}

func (uc *TagUseCase) Repository() tagrepo.TagAdminRepository {
	return uc.tags
}

func toTagResponse(t domain.QuestionTag, count int64) TagResponse {
	return TagResponse{
		ID:            t.ID.String(),
		Name:          t.Name,
		Code:          t.Code,
		Description:   t.Description,
		Color:         t.Color,
		IsActive:      t.IsActive,
		QuestionCount: count,
		CreatedAt:     t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
