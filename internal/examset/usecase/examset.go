package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	entitlementuc "virtual-exam-api/internal/entitlement/usecase"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type ExamSetUseCase struct {
	examSets     examsetrepo.Repository
	questions    questionrepo.Repository
	entitlements *entitlementuc.UseCase
	attempts     attemptrepo.Repository
	contentCache cache.CacheService
}

func NewExamSetUseCase(
	examSets examsetrepo.Repository,
	questions questionrepo.Repository,
	entitlements *entitlementuc.UseCase,
	contentCache cache.CacheService,
) *ExamSetUseCase {
	if contentCache == nil {
		contentCache = cache.Noop()
	}
	return &ExamSetUseCase{
		examSets:     examSets,
		questions:    questions,
		entitlements: entitlements,
		contentCache: contentCache,
	}
}

func NewExamSetUseCaseWithAttempts(
	examSets examsetrepo.Repository,
	questions questionrepo.Repository,
	entitlements *entitlementuc.UseCase,
	attempts attemptrepo.Repository,
	contentCache cache.CacheService,
) *ExamSetUseCase {
	uc := NewExamSetUseCase(examSets, questions, entitlements, contentCache)
	uc.attempts = attempts
	return uc
}

func (uc *ExamSetUseCase) List(ctx context.Context, filter domain.ListFilter, userID *uuid.UUID) (*domain.PaginatedResult, error) {
	filter.OnlyPublished = true

	hash := cache.HashExamSetListFilter(filter)
	key := cache.ExamSetsList(hash)
	var cached domain.PaginatedResult
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cached); ok {
		uc.enrichListAccess(ctx, &cached, userID)
		return &cached, nil
	}

	result, err := uc.examSets.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	toCache := *result
	for i := range toCache.Items {
		toCache.Items[i].Access = nil
	}
	_ = uc.contentCache.SetJSON(ctx, key, toCache, cache.TTLExamSetsList)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexExamSetsList(), key, cache.TTLExamSetsList+cache.TTLIndexBuffer)

	uc.enrichListAccess(ctx, result, userID)
	return result, nil
}

func (uc *ExamSetUseCase) enrichListAccess(ctx context.Context, result *domain.PaginatedResult, userID *uuid.UUID) {
	if uc.entitlements == nil {
		return
	}
	for i := range result.Items {
		set, err := uc.examSets.FindByCode(ctx, result.Items[i].Code)
		if err != nil || set == nil {
			continue
		}
		access := uc.entitlements.BuildAccessInfo(ctx, userID, set)
		result.Items[i].Access = &access
	}
}

func (uc *ExamSetUseCase) isPubliclyVisible(set *domain.ExamSet) bool {
	return set != nil && set.Status == domain.StatusPublished && set.IsActive
}

func (uc *ExamSetUseCase) GetByCode(ctx context.Context, code string, userID *uuid.UUID) (*domain.ExamSetSummary, error) {
	key := cache.ExamSetDetail(code)
	var cached domain.ExamSetSummary
	cacheHit := false
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cached); ok && cached.AccessType != domain.AccessPrivate {
		cacheHit = true
		set := examSetFromSummary(&cached)
		if set.AccessType == domain.AccessPrivate {
			cacheHit = false
		} else if !uc.isPubliclyVisible(set) {
			cacheHit = false
		} else {
			return uc.buildDetailSummary(ctx, set, userID, &cached)
		}
	}

	set, err := uc.examSets.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}

	summary, err := uc.buildDetailFromSet(ctx, set, userID)
	if err != nil {
		return nil, err
	}

	if !cacheHit && set.AccessType != domain.AccessPrivate && uc.isPubliclyVisible(set) {
		toCache := *summary
		toCache.Access = nil
		_ = uc.contentCache.SetJSON(ctx, key, toCache, cache.TTLExamSetDetail)
		_ = uc.contentCache.AddIndex(ctx, cache.IndexExamSet(set.ID.String()), key, cache.TTLExamSetDetail+cache.TTLIndexBuffer)
		_ = uc.contentCache.AddIndex(ctx, cache.IndexExamSetCode(set.Code), key, cache.TTLExamSetDetail+cache.TTLIndexBuffer)
	}

	return summary, nil
}

func (uc *ExamSetUseCase) buildDetailSummary(ctx context.Context, set *domain.ExamSet, userID *uuid.UUID, cached *domain.ExamSetSummary) (*domain.ExamSetSummary, error) {
	summary := *cached
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
	uc.enrichUserActivity(ctx, userID, set.ID, &summary)
	return &summary, nil
}

func (uc *ExamSetUseCase) buildDetailFromSet(ctx context.Context, set *domain.ExamSet, userID *uuid.UUID) (*domain.ExamSetSummary, error) {
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
		uc.enrichUserActivity(ctx, userID, set.ID, &summary)
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
	uc.enrichUserActivity(ctx, userID, set.ID, &summary)
	return &summary, nil
}

func (uc *ExamSetUseCase) enrichUserActivity(ctx context.Context, userID *uuid.UUID, examSetID uuid.UUID, summary *domain.ExamSetSummary) {
	if userID == nil || uc.attempts == nil {
		return
	}
	activity, err := uc.attempts.FindUserActivityForExamSet(ctx, *userID, examSetID)
	if err != nil || activity == nil || !activity.HasSubmittedAttempts {
		return
	}
	userActivity := &domain.UserExamActivitySummary{
		HasSubmittedAttempts: true,
		LatestScorePercent:   activity.LatestScorePercent,
	}
	if activity.LatestSubmittedAttemptID != nil {
		id := activity.LatestSubmittedAttemptID.String()
		userActivity.LatestSubmittedAttemptID = &id
	}
	summary.UserActivity = userActivity
}

func examSetFromSummary(summary *domain.ExamSetSummary) *domain.ExamSet {
	id, _ := uuid.Parse(summary.ID)
	return &domain.ExamSet{
		ID:                  id,
		Code:                summary.Code,
		Title:               summary.Title,
		Description:         summary.Description,
		CoverImageURL:       summary.CoverImageURL,
		DurationMinutes:     summary.DurationMinutes,
		TotalQuestions:      summary.TotalQuestions,
		PassingScore:        summary.PassingScore,
		Difficulty:          summary.Difficulty,
		AccessType:          summary.AccessType,
		AllowSinglePurchase: summary.AllowSinglePurchase,
		PriceAmount:         summary.PriceAmount,
		OriginalPriceAmount: summary.OriginalPriceAmount,
		Currency:            summary.Currency,
		SalePriceAmount:     summary.SalePriceAmount,
		Mode:                summary.Mode,
		IsOfficial:          summary.IsOfficial,
		IsFeatured:          summary.IsFeatured,
		IsActive:            summary.IsActive,
		Status:              summary.Status,
		AnswerSheetLayout:   summary.AnswerSheetLayout,
		ExamTrack:           summary.ExamTrack,
	}
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
