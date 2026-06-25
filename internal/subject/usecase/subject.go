package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/question/domain"
	subjectrepo "virtual-exam-api/internal/subject/repository"
)

type SubjectUseCase struct {
	subjects subjectrepo.SubjectAdminRepository
}

func NewSubjectUseCase(subjects subjectrepo.SubjectAdminRepository) *SubjectUseCase {
	return &SubjectUseCase{subjects: subjects}
}

type SubjectInput struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

type SubjectResponse struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	QuestionCount int64  `json:"question_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type SubjectListResponse struct {
	Items      []SubjectResponse `json:"items"`
	TotalItems int64             `json:"total_items"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
}

func (uc *SubjectUseCase) List(ctx context.Context, filter subjectrepo.SubjectAdminFilter) (*SubjectListResponse, error) {
	items, total, err := uc.subjects.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]SubjectResponse, len(items))
	for i, s := range items {
		count, _ := uc.subjects.CountQuestions(ctx, s.ID)
		resp[i] = toSubjectResponse(s, count)
	}
	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return &SubjectListResponse{Items: resp, TotalItems: total, Page: page, Limit: limit}, nil
}

func (uc *SubjectUseCase) Get(ctx context.Context, id uuid.UUID) (*SubjectResponse, error) {
	s, err := uc.subjects.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, apperrors.ErrNotFound
	}
	count, _ := uc.subjects.CountQuestions(ctx, s.ID)
	resp := toSubjectResponse(*s, count)
	return &resp, nil
}

func (uc *SubjectUseCase) Create(ctx context.Context, input SubjectInput) (*SubjectResponse, error) {
	if input.Name == "" || input.Code == "" {
		return nil, apperrors.ErrInvalidInput
	}
	if !subjectrepo.IsValidSubjectCode(input.Code) {
		return nil, apperrors.ErrInvalidInput
	}
	existing, err := uc.subjects.FindByCode(ctx, input.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.ErrCodeTaken
	}
	subject := domain.Subject{Name: input.Name, Code: input.Code, Description: input.Description}
	if err := uc.subjects.Create(ctx, &subject); err != nil {
		return nil, err
	}
	resp := toSubjectResponse(subject, 0)
	return &resp, nil
}

func (uc *SubjectUseCase) Update(ctx context.Context, id uuid.UUID, input SubjectInput) (*SubjectResponse, error) {
	s, err := uc.subjects.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, apperrors.ErrNotFound
	}
	if input.Name == "" || input.Code == "" {
		return nil, apperrors.ErrInvalidInput
	}
	if input.Code != s.Code {
		existing, err := uc.subjects.FindByCode(ctx, input.Code)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, apperrors.ErrCodeTaken
		}
	}
	s.Name = input.Name
	s.Code = input.Code
	s.Description = input.Description
	if err := uc.subjects.Update(ctx, s); err != nil {
		return nil, err
	}
	count, _ := uc.subjects.CountQuestions(ctx, s.ID)
	resp := toSubjectResponse(*s, count)
	return &resp, nil
}

func (uc *SubjectUseCase) Delete(ctx context.Context, id uuid.UUID) error {
	s, err := uc.subjects.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if s == nil {
		return apperrors.ErrNotFound
	}
	if err := uc.subjects.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			return apperrors.ErrSubjectHasQuestions
		}
		return err
	}
	return nil
}

func toSubjectResponse(s domain.Subject, count int64) SubjectResponse {
	return SubjectResponse{
		ID:            s.ID.String(),
		Code:          s.Code,
		Name:          s.Name,
		Description:   s.Description,
		QuestionCount: count,
		CreatedAt:     s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
