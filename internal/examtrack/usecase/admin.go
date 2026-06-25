package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examtrack/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type AdminUseCase struct {
	tracks trackrepo.AdminRepository
	reads  trackrepo.Repository
}

func NewAdminUseCase(tracks trackrepo.AdminRepository, reads trackrepo.Repository) *AdminUseCase {
	return &AdminUseCase{tracks: tracks, reads: reads}
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

type TrackListResponse struct {
	Items      []TrackAdminResponse `json:"items"`
	TotalItems int64                `json:"total_items"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
}

func (uc *AdminUseCase) List(ctx context.Context, filter trackrepo.AdminFilter) (*TrackListResponse, error) {
	items, total, err := uc.tracks.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]TrackAdminResponse, len(items))
	for i, t := range items {
		resp[i] = toTrackAdminResponse(t)
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	return &TrackListResponse{Items: resp, TotalItems: total, Page: page, Limit: limit}, nil
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
	return uc.tracks.Delete(ctx, id)
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
