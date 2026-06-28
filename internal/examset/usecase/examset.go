package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	entitlementuc "virtual-exam-api/internal/entitlement/usecase"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type ExamSetUseCase struct {
	examSets      examsetrepo.Repository
	questions     questionrepo.Repository
	entitlements  *entitlementuc.UseCase
}

func NewExamSetUseCase(
	examSets examsetrepo.Repository,
	questions questionrepo.Repository,
	entitlements *entitlementuc.UseCase,
) *ExamSetUseCase {
	return &ExamSetUseCase{
		examSets:     examSets,
		questions:    questions,
		entitlements: entitlements,
	}
}

func (uc *ExamSetUseCase) List(ctx context.Context, filter domain.ListFilter, userID *uuid.UUID) (*domain.PaginatedResult, error) {
	filter.OnlyPublished = true
	result, err := uc.examSets.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if uc.entitlements != nil {
		for i := range result.Items {
			set, err := uc.examSets.FindByCode(ctx, result.Items[i].Code)
			if err != nil || set == nil {
				continue
			}
			access := uc.entitlements.BuildAccessInfo(ctx, userID, set)
			result.Items[i].Access = &access
		}
	}
	return result, nil
}

func (uc *ExamSetUseCase) isPubliclyVisible(set *domain.ExamSet) bool {
	return set != nil && set.Status == domain.StatusPublished && set.IsActive
}

func (uc *ExamSetUseCase) GetByCode(ctx context.Context, code string, userID *uuid.UUID) (*domain.ExamSetSummary, error) {
	set, err := uc.examSets.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if set.AccessType == domain.AccessPrivate {
		if set.Status != domain.StatusPublished || !set.IsActive {
			return nil, apperrors.ErrExamSetNotFound
		}
		if uc.entitlements == nil {
			return nil, apperrors.ErrPrivateExamAccessRequired
		}
		check := uc.entitlements.CheckExamSetAccess(ctx, userID, set)
		if !check.CanStart {
			return nil, apperrors.ErrPrivateExamAccessRequired
		}
		summary := set.ToSummary()
		questionCount := -1
		if uc.questions != nil {
			if questions, err := uc.questions.ListByExamSetID(ctx, set.ID); err == nil {
				questionCount = len(questions)
			}
		}
		access := uc.entitlements.BuildAccessInfoWithQuestionCount(ctx, userID, set, questionCount)
		summary.Access = &access
		return &summary, nil
	}
	if !uc.isPubliclyVisible(set) {
		return nil, apperrors.ErrExamSetNotFound
	}
	summary := set.ToSummary()
	if uc.entitlements != nil {
		questionCount := -1
		if uc.questions != nil {
			if questions, err := uc.questions.ListByExamSetID(ctx, set.ID); err == nil {
				questionCount = len(questions)
			}
		}
		access := uc.entitlements.BuildAccessInfoWithQuestionCount(ctx, userID, set, questionCount)
		summary.Access = &access
	}
	return &summary, nil
}

func (uc *ExamSetUseCase) GetByCodeForPreview(ctx context.Context, code string, userID *uuid.UUID) (*domain.ExamSet, error) {
	set, err := uc.examSets.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if set.AccessType == domain.AccessPrivate {
		if set.Status != domain.StatusPublished || !set.IsActive {
			return nil, apperrors.ErrExamSetNotFound
		}
		if uc.entitlements == nil {
			return nil, apperrors.ErrPrivateExamAccessRequired
		}
		check := uc.entitlements.CheckExamSetAccess(ctx, userID, set)
		if !check.CanStart {
			return nil, apperrors.ErrPrivateExamAccessRequired
		}
		return set, nil
	}
	if !uc.isPubliclyVisible(set) {
		return nil, apperrors.ErrExamSetNotFound
	}
	return set, nil
}

func (uc *ExamSetUseCase) QuestionsPreview(ctx context.Context, code string, userID *uuid.UUID) (*domain.QuestionsPreviewResponse, error) {
	set, err := uc.GetByCodeForPreview(ctx, code, userID)
	if err != nil {
		return nil, err
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
