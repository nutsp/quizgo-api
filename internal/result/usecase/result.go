package usecase

import (
	"context"
	"math"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/result/domain"
	resultrepo "virtual-exam-api/internal/result/repository"
)

type ResultUseCase struct {
	repo resultrepo.Repository
}

func NewResultUseCase(repo resultrepo.Repository) *ResultUseCase {
	return &ResultUseCase{repo: repo}
}

func (uc *ResultUseCase) GetMyResultsSummary(ctx context.Context, userID uuid.UUID) (*domain.OverallSummary, error) {
	stats, err := uc.repo.GetOverallStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary := &domain.OverallSummary{
		TotalAttempts:          stats.TotalAttempts,
		CompletedExamSets:      stats.CompletedExamSets,
		CompletedExamTracks:    stats.CompletedExamTracks,
		AverageScorePercent:    round1(derefFloat(stats.AverageScorePercent)),
		BestScorePercent:       round1(derefFloat(stats.BestScorePercent)),
		LatestScorePercent:     round1(derefFloat(stats.LatestScorePercent)),
		PassedAttempts:         stats.PassedAttempts,
		FailedAttempts:         stats.FailedAttempts,
		AverageDurationSeconds: round1(derefFloat(stats.AverageDurationSeconds)),
		WeakSubjects:           []domain.WeakSubject{},
	}

	if summary.TotalAttempts > 0 {
		summary.PassRatePercent = round1(float64(summary.PassedAttempts) / float64(summary.TotalAttempts) * 100)
	}

	practice, err := uc.repo.GetMostPracticedTrack(ctx, userID)
	if err != nil {
		return nil, err
	}
	if practice != nil {
		summary.MostPracticedExamTrack = &domain.ExamTrackRef{
			Code: practice.TrackCode,
			Name: practice.TrackName,
		}
	}

	weak, err := uc.repo.ListWeakSubjects(ctx, userID, nil, 5)
	if err != nil {
		return nil, err
	}
	for _, w := range weak {
		summary.WeakSubjects = append(summary.WeakSubjects, domain.WeakSubject{
			SubjectCode:         w.SubjectCode,
			SubjectName:         w.SubjectName,
			AverageScorePercent: round1(w.AverageScorePercent),
		})
	}

	return summary, nil
}

func (uc *ResultUseCase) GetMyExamTrackResults(ctx context.Context, userID uuid.UUID) ([]domain.ExamTrackSummaryItem, error) {
	trackIDs, err := uc.repo.ListTracksWithAttempts(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.ExamTrackSummaryItem, 0, len(trackIDs))
	for _, trackID := range trackIDs {
		item, err := uc.buildTrackSummary(ctx, userID, trackID)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

func (uc *ResultUseCase) GetMyExamTrackResultDetail(ctx context.Context, userID uuid.UUID, trackCode string) (*domain.ExamTrackDetailResponse, error) {
	track, err := uc.repo.FindTrackByCode(ctx, trackCode)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, apperrors.ErrExamTrackNotFound
	}

	summaryItem, err := uc.buildTrackSummary(ctx, userID, track.ID)
	if err != nil {
		return nil, err
	}

	resp := &domain.ExamTrackDetailResponse{
		ExamTrack: domain.ExamTrackRef{
			ID:            track.ID.String(),
			Code:          track.Code,
			Name:          track.Name,
			Description:   track.Description,
			CoverImageURL: track.CoverImageURL,
		},
		ExamSets:         []domain.ExamSetProgressItem{},
		WeaknessAnalysis: []domain.WeakSubject{},
	}

	if summaryItem != nil {
		resp.Summary = domain.TrackSummaryStats{
			CompletedExamSets:       summaryItem.CompletedExamSets,
			TotalExamSets:           summaryItem.TotalExamSets,
			TotalAttempts:           summaryItem.TotalAttempts,
			AverageBestScorePercent: summaryItem.AverageBestScorePercent,
			BestScorePercent:        summaryItem.BestScorePercent,
			LatestScorePercent:      summaryItem.LatestScorePercent,
			PassedExamSets:          summaryItem.PassedExamSets,
			FailedExamSets:          summaryItem.FailedExamSets,
			AverageDurationSeconds:  summaryItem.AverageDurationSeconds,
			ReadinessPercent:        summaryItem.AverageBestScorePercent,
		}
	}

	sets, err := uc.repo.ListActiveExamSetsByTrack(ctx, track.ID)
	if err != nil {
		return nil, err
	}

	for _, set := range sets {
		attempts, err := uc.repo.ListAttemptsByExamSet(ctx, userID, set.ID)
		if err != nil {
			return nil, err
		}
		if len(attempts) == 0 {
			continue
		}
		resp.ExamSets = append(resp.ExamSets, buildExamSetProgress(set, attempts))
	}

	weak, err := uc.repo.ListWeakSubjects(ctx, userID, &track.ID, 5)
	if err != nil {
		return nil, err
	}
	for _, w := range weak {
		resp.WeaknessAnalysis = append(resp.WeaknessAnalysis, domain.WeakSubject{
			SubjectName:         w.SubjectName,
			AverageScorePercent: round1(w.AverageScorePercent),
			Recommendation:      "ควรฝึกข้อสอบหมวดนี้เพิ่ม",
		})
	}

	return resp, nil
}

func (uc *ResultUseCase) ListMyAttemptResults(ctx context.Context, userID uuid.UUID, filter domain.AttemptHistoryFilter) (*domain.PaginatedAttempts, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}

	rows, total, err := uc.repo.ListAttempts(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	attemptNos := uc.computeAttemptNumbers(ctx, userID, rows)

	items := make([]domain.AttemptHistoryItem, len(rows))
	for i, row := range rows {
		items[i] = toAttemptHistoryItem(row, attemptNos[row.ID])
	}

	return &domain.PaginatedAttempts{
		Items: items,
		Pagination: domain.Pagination{
			Page:  filter.Page,
			Limit: filter.Limit,
			Total: total,
		},
	}, nil
}

func (uc *ResultUseCase) GetMyExamSetResultDetail(ctx context.Context, userID uuid.UUID, examSetCode string) (*domain.ExamSetDetailResponse, error) {
	set, err := uc.repo.FindExamSetByCode(ctx, examSetCode)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}

	rows, err := uc.repo.ListAttemptsByExamSet(ctx, userID, set.ID)
	if err != nil {
		return nil, err
	}

	resp := &domain.ExamSetDetailResponse{
		ExamSet: domain.ExamSetRef{
			Code:          set.Code,
			Title:         set.Title,
			CoverImageURL: set.CoverImageURL,
			PassingScore:  set.PassingScore,
		},
		Attempts: []domain.ExamSetAttemptItem{},
	}

	if len(rows) == 0 {
		return resp, nil
	}

	progress := buildExamSetProgress(*set, rows)
	resp.Summary = domain.ExamSetSummaryStats{
		AttemptCount:           progress.AttemptCount,
		FirstScorePercent:      progress.FirstScorePercent,
		LatestScorePercent:     progress.LatestScorePercent,
		BestScorePercent:       progress.BestScorePercent,
		ImprovementPercent:     progress.ImprovementPercent,
		Passed:                 progress.Passed,
		AverageDurationSeconds: averageDuration(rows),
	}

	for i, row := range rows {
		resp.Attempts = append(resp.Attempts, domain.ExamSetAttemptItem{
			AttemptID:       row.ID.String(),
			AttemptNo:       i + 1,
			ScorePercent:    round1(row.ScorePercent),
			Passed:          row.ScorePercent >= float64(row.SetPassingScore),
			DurationSeconds: derefInt(row.DurationSeconds),
			SubmittedAt:     row.SubmittedAt,
		})
	}

	return resp, nil
}

func (uc *ResultUseCase) buildTrackSummary(ctx context.Context, userID, trackID uuid.UUID) (*domain.ExamTrackSummaryItem, error) {
	trackRow, err := uc.repo.FindTrackByID(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if trackRow == nil {
		return nil, nil
	}

	totalSets, err := uc.repo.CountActiveExamSetsByTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}

	bestAttempts, err := uc.repo.GetTrackBestAttempts(ctx, userID, trackID)
	if err != nil {
		return nil, err
	}

	totalAttempts, err := uc.repo.CountAttemptsByTrack(ctx, userID, trackID)
	if err != nil {
		return nil, err
	}

	item := &domain.ExamTrackSummaryItem{
		ExamTrack: domain.ExamTrackRef{
			ID:            trackRow.ID.String(),
			Code:          trackRow.Code,
			Name:          trackRow.Name,
			CoverImageURL: trackRow.CoverImageURL,
		},
		CompletedExamSets: len(bestAttempts),
		TotalExamSets:     int(totalSets),
		TotalAttempts:     totalAttempts,
		WeakSubjects:      []domain.WeakSubject{},
	}

	if len(bestAttempts) == 0 {
		return item, nil
	}

	var sumBest, maxBest float64
	var sumDuration float64
	var durationCount int
	passedSets := 0

	for _, ba := range bestAttempts {
		sumBest += ba.ScorePercent
		if ba.ScorePercent > maxBest {
			maxBest = ba.ScorePercent
		}
		if ba.ScorePercent >= float64(ba.PassingScore) {
			passedSets++
		}
		if ba.DurationSeconds != nil {
			sumDuration += float64(*ba.DurationSeconds)
			durationCount++
		}
		if ba.SubmittedAt != nil {
			if item.LastAttemptAt == nil || ba.SubmittedAt.After(*item.LastAttemptAt) {
				t := *ba.SubmittedAt
				item.LastAttemptAt = &t
			}
		}
	}

	item.AverageBestScorePercent = round1(sumBest / float64(len(bestAttempts)))
	item.BestScorePercent = round1(maxBest)
	item.PassedExamSets = passedSets
	item.FailedExamSets = len(bestAttempts) - passedSets
	if durationCount > 0 {
		item.AverageDurationSeconds = round1(sumDuration / float64(durationCount))
	}

	if latest, ok, err := uc.repo.GetLatestAttemptScoreByTrack(ctx, userID, trackID); err != nil {
		return nil, err
	} else if ok {
		item.LatestScorePercent = round1(latest)
	}

	weak, err := uc.repo.ListWeakSubjects(ctx, userID, &trackID, 3)
	if err != nil {
		return nil, err
	}
	for _, w := range weak {
		item.WeakSubjects = append(item.WeakSubjects, domain.WeakSubject{
			SubjectName:         w.SubjectName,
			AverageScorePercent: round1(w.AverageScorePercent),
		})
	}

	return item, nil
}

func (uc *ResultUseCase) computeAttemptNumbers(ctx context.Context, userID uuid.UUID, rows []resultrepo.AttemptRow) map[uuid.UUID]int {
	result := make(map[uuid.UUID]int, len(rows))
	setAttempts := make(map[uuid.UUID][]resultrepo.AttemptRow)

	for _, row := range rows {
		setAttempts[row.ExamSetID] = append(setAttempts[row.ExamSetID], row)
	}

	for setID := range setAttempts {
		all, err := uc.repo.ListAttemptsByExamSet(ctx, userID, setID)
		if err != nil {
			continue
		}
		for i, a := range all {
			result[a.ID] = i + 1
		}
	}
	return result
}

func buildExamSetProgress(set resultrepo.ExamSetRow, attempts []resultrepo.AttemptRow) domain.ExamSetProgressItem {
	first := attempts[0]
	latest := attempts[len(attempts)-1]

	best := attempts[0]
	for _, a := range attempts {
		if a.ScorePercent > best.ScorePercent {
			best = a
		} else if a.ScorePercent == best.ScorePercent {
			if derefInt(a.DurationSeconds) < derefInt(best.DurationSeconds) {
				best = a
			}
		}
	}

	return domain.ExamSetProgressItem{
		ExamSet: domain.ExamSetRef{
			ID:              set.ID.String(),
			Code:            set.Code,
			Title:           set.Title,
			CoverImageURL:   set.CoverImageURL,
			TotalQuestions:  set.TotalQuestions,
			DurationMinutes: set.DurationMinutes,
			PassingScore:    set.PassingScore,
		},
		AttemptCount:       len(attempts),
		LatestAttemptID:    latest.ID.String(),
		LatestScorePercent: round1(latest.ScorePercent),
		BestAttemptID:      best.ID.String(),
		BestScorePercent:   round1(best.ScorePercent),
		FirstScorePercent:  round1(first.ScorePercent),
		ImprovementPercent: round1(best.ScorePercent - first.ScorePercent),
		Passed:             best.ScorePercent >= float64(set.PassingScore),
		LastAttemptAt:      latest.SubmittedAt,
	}
}

func toAttemptHistoryItem(row resultrepo.AttemptRow, attemptNo int) domain.AttemptHistoryItem {
	return domain.AttemptHistoryItem{
		AttemptID: row.ID.String(),
		ExamTrack: domain.ExamTrackRef{
			Code: row.TrackCode,
			Name: row.TrackName,
		},
		ExamSet: domain.ExamSetRef{
			Code:          row.SetCode,
			Title:         row.SetTitle,
			CoverImageURL: row.SetCoverURL,
		},
		AttemptNo:       attemptNo,
		Score:           row.Score,
		TotalScore:      row.TotalScore,
		ScorePercent:    round1(row.ScorePercent),
		Passed:          row.ScorePercent >= float64(row.SetPassingScore),
		CorrectCount:    row.CorrectCount,
		WrongCount:      row.WrongCount,
		UnansweredCount: row.UnansweredCount,
		DurationSeconds: derefInt(row.DurationSeconds),
		Status:          row.Status,
		StartedAt:       row.StartedAt,
		SubmittedAt:     row.SubmittedAt,
	}
}

func averageDuration(rows []resultrepo.AttemptRow) float64 {
	if len(rows) == 0 {
		return 0
	}
	var sum float64
	var count int
	for _, r := range rows {
		if r.DurationSeconds != nil {
			sum += float64(*r.DurationSeconds)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return round1(sum / float64(count))
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
