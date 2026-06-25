package usecase

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	"virtual-exam-api/internal/home/domain"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
)

type HomeUseCase struct {
	tracks    trackrepo.Repository
	examSets  examsetrepo.Repository
	attempts  attemptrepo.Repository
}

func NewHomeUseCase(
	tracks trackrepo.Repository,
	examSets examsetrepo.Repository,
	attempts attemptrepo.Repository,
) *HomeUseCase {
	return &HomeUseCase{
		tracks:   tracks,
		examSets: examSets,
		attempts: attempts,
	}
}

func (uc *HomeUseCase) GetHome(ctx context.Context, userID *uuid.UUID) (*domain.HomeResponse, error) {
	tracks, err := uc.tracks.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	recommended := make([]domain.ExamTrackItem, 0, len(tracks))
	for _, t := range tracks {
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

	popularSets, err := uc.examSets.ListPopular(ctx, 4)
	if err != nil {
		return nil, err
	}

	popular := make([]domain.ExamSetItem, 0, len(popularSets))
	for _, s := range popularSets {
		item := domain.ExamSetItem{
			ID:              s.ID.String(),
			Code:            s.Code,
			Title:           s.Title,
			Description:     s.Description,
			CoverImageURL:   s.CoverImageURL,
			DurationMinutes: s.DurationMinutes,
			TotalQuestions:  s.TotalQuestions,
			PassingScore:    s.PassingScore,
			Difficulty:      s.Difficulty,
			AccessType:      s.AccessType,
			PriceAmount:     s.PriceAmount,
			Currency:        s.Currency,
			SalePriceAmount: s.SalePriceAmount,
			Mode:            s.Mode,
			IsOfficial:      s.IsOfficial,
			IsFeatured:      s.IsFeatured,
		}
		if s.ExamTrack != nil {
			item.ExamTrack = &domain.ExamTrackRef{
				Code: s.ExamTrack.Code,
				Name: s.ExamTrack.Name,
			}
		}
		popular = append(popular, item)
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