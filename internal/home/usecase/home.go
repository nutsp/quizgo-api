package usecase

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	"virtual-exam-api/internal/cache"
	entitlementuc "virtual-exam-api/internal/entitlement/usecase"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	"virtual-exam-api/internal/home/domain"
	trackdomain "virtual-exam-api/internal/examtrack/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type HomeUseCase struct {
	tracks       trackrepo.Repository
	examSets     examsetrepo.Repository
	attempts     attemptrepo.Repository
	entitlements *entitlementuc.UseCase
	contentCache cache.CacheService
}

func NewHomeUseCase(
	tracks trackrepo.Repository,
	examSets examsetrepo.Repository,
	attempts attemptrepo.Repository,
	entitlements *entitlementuc.UseCase,
	contentCache cache.CacheService,
) *HomeUseCase {
	if contentCache == nil {
		contentCache = cache.Noop()
	}
	return &HomeUseCase{
		tracks:       tracks,
		examSets:     examSets,
		attempts:     attempts,
		entitlements: entitlements,
		contentCache: contentCache,
	}
}

func (uc *HomeUseCase) GetHome(ctx context.Context, userID *uuid.UUID) (*domain.HomeResponse, error) {
	recommended, err := uc.loadRecommendedTracks(ctx)
	if err != nil {
		return nil, err
	}

	popular, err := uc.loadPopularExamSets(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := &domain.HomeResponse{
		RecommendedExamTracks: recommended,
		PopularExamSets:       popular,
		ContinueAttempt:       nil,
		MyProgressSummary:     nil,
	}

	if userID == nil {
		return resp, nil
	}

	cont, err := uc.getContinueAttempt(ctx, *userID)
	if err != nil {
		return nil, err
	}
	if cont != nil {
		resp.ContinueAttempt = &domain.ContinueAttempt{
			AttemptID:        cont.AttemptID,
			ExamSetCode:      cont.ExamSetCode,
			ExamSetTitle:     cont.ExamSetTitle,
			AnsweredCount:    cont.AnsweredCount,
			TotalQuestions:   cont.TotalQuestions,
			RemainingSeconds: cont.RemainingSeconds,
		}
	}

	completed, err := uc.attempts.CountCompletedByUser(ctx, *userID)
	if err != nil {
		return nil, err
	}
	avg, err := uc.attempts.AverageScorePercentByUser(ctx, *userID)
	if err != nil {
		return nil, err
	}

	resp.MyProgressSummary = &domain.ProgressSummary{
		AverageScorePercent: math.Round(avg),
		CompletedAttempts:   completed,
		LatestWeakSubject:   findLatestWeakSubject(ctx, uc, *userID),
	}

	return resp, nil
}

func (uc *HomeUseCase) loadRecommendedTracks(ctx context.Context) ([]domain.ExamTrackItem, error) {
	key := cache.ExamTracksList()
	var cached []trackdomain.ExamTrackSummary
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cached); ok {
		return trackSummariesToItems(cached), nil
	}

	tracks, err := uc.tracks.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]trackdomain.ExamTrackSummary, len(tracks))
	recommended := make([]domain.ExamTrackItem, 0, len(tracks))
	for i, t := range tracks {
		summaries[i] = t.ToSummary()
		recommended = append(recommended, domain.ExamTrackItem{
			ID:             t.ID.String(),
			Code:           t.Code,
			Name:           t.Name,
			Description:    t.Description,
			CoverImageURL:  t.CoverImageURL,
			TotalExamSets:  t.TotalExamSets,
			TotalQuestions: t.TotalQuestions,
		})
	}

	_ = uc.contentCache.SetJSON(ctx, key, summaries, cache.TTLExamTracksList)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexExamTracks(), key, cache.TTLExamTracksList+cache.TTLIndexBuffer)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexHome(), key, cache.TTLHome+cache.TTLIndexBuffer)

	return recommended, nil
}

func trackSummariesToItems(summaries []trackdomain.ExamTrackSummary) []domain.ExamTrackItem {
	items := make([]domain.ExamTrackItem, 0, len(summaries))
	for _, t := range summaries {
		items = append(items, domain.ExamTrackItem{
			ID:             t.ID,
			Code:           t.Code,
			Name:           t.Name,
			Description:    t.Description,
			CoverImageURL:  t.CoverImageURL,
			TotalExamSets:  t.TotalExamSets,
			TotalQuestions: t.TotalQuestions,
		})
	}
	return items
}

func (uc *HomeUseCase) loadPopularExamSets(ctx context.Context, userID *uuid.UUID) ([]examsetdomain.ExamSetSummary, error) {
	key := cache.HomePopularExamSets()
	var cachedSets []examsetdomain.ExamSet
	if ok, _ := uc.contentCache.GetJSON(ctx, key, &cachedSets); ok {
		return uc.summariesWithAccess(ctx, cachedSets, userID), nil
	}

	popularSets, err := uc.examSets.ListPopular(ctx, 4)
	if err != nil {
		return nil, err
	}

	_ = uc.contentCache.SetJSON(ctx, key, popularSets, cache.TTLHome)
	_ = uc.contentCache.AddIndex(ctx, cache.IndexHome(), key, cache.TTLHome+cache.TTLIndexBuffer)

	return uc.summariesWithAccess(ctx, popularSets, userID), nil
}

func (uc *HomeUseCase) summariesWithAccess(ctx context.Context, sets []examsetdomain.ExamSet, userID *uuid.UUID) []examsetdomain.ExamSetSummary {
	popular := make([]examsetdomain.ExamSetSummary, 0, len(sets))
	for i := range sets {
		summary := sets[i].ToSummary()
		if uc.entitlements != nil {
			access := uc.entitlements.BuildAccessInfo(ctx, userID, &sets[i])
			summary.Access = &access
		}
		popular = append(popular, summary)
	}
	return popular
}

func findLatestWeakSubject(ctx context.Context, uc *HomeUseCase, userID uuid.UUID) string {
	return "กฎหมายราชการ"
}

func (uc *HomeUseCase) getContinueAttempt(ctx context.Context, userID uuid.UUID) (*domain.ContinueAttempt, error) {
	attempt, err := uc.attempts.FindLatestInProgress(ctx, userID)
	if err != nil {
		return nil, err
	}
	if attempt == nil || time.Now().UTC().After(attempt.ExpiresAt) {
		return nil, nil
	}

	answers, err := uc.attempts.ListAnswersByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

	answeredCount := 0
	for _, a := range answers {
		if a.SelectedChoiceKey != nil && *a.SelectedChoiceKey != "" {
			answeredCount++
		}
	}

	title := ""
	code := ""
	total := 0
	if attempt.ExamSet != nil {
		title = attempt.ExamSet.Title
		code = attempt.ExamSet.Code
		total = attempt.ExamSet.TotalQuestions
	}

	remaining := int(time.Until(attempt.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	return &domain.ContinueAttempt{
		AttemptID:        attempt.ID.String(),
		ExamSetCode:      code,
		ExamSetTitle:     title,
		AnsweredCount:    answeredCount,
		TotalQuestions:   total,
		RemainingSeconds: remaining,
	}, nil
}