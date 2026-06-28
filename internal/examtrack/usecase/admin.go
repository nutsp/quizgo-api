package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/examtrack/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type AdminUseCase struct {
	tracks      trackrepo.AdminRepository
	reads       trackrepo.Repository
	invalidator *cache.Invalidator
}

func NewAdminUseCase(tracks trackrepo.AdminRepository, reads trackrepo.Repository, invalidator *cache.Invalidator) *AdminUseCase {
	return &AdminUseCase{tracks: tracks, reads: reads, invalidator: invalidator}
}

type CreateTrackInput struct {
	Name          string  `json:"name"`
	Code          string  `json:"code"`
	Description   string  `json:"description"`
	CoverImageURL *string `json:"cover_image_url"`
	IsActive      bool    `json:"is_active"`
}

type UpdateTrackInput = CreateTrackInput

type TrackAdminResponse struct {
	ID             string  `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	CoverImageURL  *string `json:"cover_image_url,omitempty"`
	TotalExamSets  int     `json:"total_exam_sets"`
	TotalQuestions int     `json:"total_questions"`
	TotalAttempts  int     `json:"total_attempts"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type TrackListResponse = pagination.PaginatedList[TrackAdminResponse]

func (uc *AdminUseCase) List(ctx context.Context, filter trackrepo.AdminFilter) (*TrackListResponse, error) {
	items, total, err := uc.tracks.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]TrackAdminResponse, len(items))
	for i, t := range items {
		resp[i] = toTrackAdminResponse(t)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func (uc *AdminUseCase) Get(ctx context.Context, id uuid.UUID) (*TrackAdminResponse, error) {
	track, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, apperrors.ErrExamTrackNotFound
	}
	resp := toTrackAdminResponse(*track)
	return &resp, nil
}

func (uc *AdminUseCase) Create(ctx context.Context, input CreateTrackInput) (*TrackAdminResponse, error) {
	if input.Name == "" || input.Code == "" {
		return nil, apperrors.ErrInvalidInput
	}
	if !trackrepo.IsValidTrackCode(input.Code) {
		return nil, apperrors.ErrInvalidInput
	}
	existing, err := uc.reads.FindByCode(ctx, input.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.ErrCodeTaken
	}
	track := domain.ExamTrack{
		Code:          input.Code,
		Name:          input.Name,
		Description:   input.Description,
		CoverImageURL: input.CoverImageURL,
		IsActive:      input.IsActive,
	}
	if err := uc.tracks.Create(ctx, &track); err != nil {
		return nil, err
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamTrackChanged(ctx)
	}
	resp := toTrackAdminResponse(track)
	return &resp, nil
}

func (uc *AdminUseCase) Update(ctx context.Context, id uuid.UUID, input UpdateTrackInput) (*TrackAdminResponse, error) {
	track, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, apperrors.ErrExamTrackNotFound
	}
	if input.Name == "" || input.Code == "" {
		return nil, apperrors.ErrInvalidInput
	}
	if !trackrepo.IsValidTrackCode(input.Code) {
		return nil, apperrors.ErrInvalidInput
	}
	if input.Code != track.Code {
		existing, err := uc.reads.FindByCode(ctx, input.Code)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, apperrors.ErrCodeTaken
		}
	}
	track.Name = input.Name
	track.Code = input.Code
	track.Description = input.Description
	track.CoverImageURL = input.CoverImageURL
	track.IsActive = input.IsActive
	if err := uc.tracks.Update(ctx, track); err != nil {
		return nil, err
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamTrackChanged(ctx)
	}
	resp := toTrackAdminResponse(*track)
	return &resp, nil
}

func (uc *AdminUseCase) Delete(ctx context.Context, id uuid.UUID) (deactivated bool, err error) {
	track, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	if track == nil {
		return false, apperrors.ErrExamTrackNotFound
	}
	deactivated, err = uc.tracks.Delete(ctx, id)
	if err == nil && uc.invalidator != nil {
		uc.invalidator.OnExamTrackChanged(ctx)
	}
	return deactivated, err
}

func toTrackAdminResponse(t domain.ExamTrack) TrackAdminResponse {
	return TrackAdminResponse{
		ID:             t.ID.String(),
		Code:           t.Code,
		Name:           t.Name,
		Description:    t.Description,
		CoverImageURL:  t.CoverImageURL,
		TotalExamSets:  t.TotalExamSets,
		TotalQuestions: t.TotalQuestions,
		TotalAttempts:  t.TotalAttempts,
		IsActive:       t.IsActive,
		CreatedAt:      t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
