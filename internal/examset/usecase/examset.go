package usecase

import (
	"context"

	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type ExamSetUseCase struct {
	examSets  examsetrepo.Repository
	questions questionrepo.Repository
}

func NewExamSetUseCase(examSets examsetrepo.Repository, questions questionrepo.Repository) *ExamSetUseCase {
	return &ExamSetUseCase{examSets: examSets, questions: questions}
}

func (uc *ExamSetUseCase) List(ctx context.Context, filter domain.ListFilter) (*domain.PaginatedResult, error) {
	filter.OnlyActive = true
	return uc.examSets.List(ctx, filter)
}

func (uc *ExamSetUseCase) GetByCode(ctx context.Context, code string) (*domain.ExamSetSummary, error) {
	set, err := uc.examSets.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if set == nil || !set.IsActive {
		return nil, apperrors.ErrExamSetNotFound
	}
	summary := set.ToSummary()
	return &summary, nil
}

func (uc *ExamSetUseCase) QuestionsPreview(ctx context.Context, code string) (*domain.QuestionsPreviewResponse, error) {
	set, err := uc.examSets.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if set == nil || !set.IsActive {
		return nil, apperrors.ErrExamSetNotFound
	}

	questions, err := uc.questions.ListPreviewByExamSetID(ctx, set.ID)
	if err != nil {
		return nil, err
	}

	previews := make([]domain.QuestionPreview, 0, len(questions))
	for _, q := range questions {
		preview := domain.QuestionPreview{
			QuestionNo: q.QuestionNo,
			QuestionID: q.QuestionID.String(),
		}
		if q.Question != nil {
			preview.QuestionText = truncateText(q.Question.QuestionText, 120)
			preview.Difficulty = q.Question.Difficulty
			if q.Question.Subject != nil {
				preview.SubjectName = q.Question.Subject.Name
			}
		}
		previews = append(previews, preview)
	}

	summary := set.ToSummary()
	return &domain.QuestionsPreviewResponse{
		ExamSet:   summary,
		Questions: previews,
	}, nil
}

func truncateText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
