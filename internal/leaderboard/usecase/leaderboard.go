package usecase

import (
	"context"
	"math"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	userdomain "virtual-exam-api/internal/user/domain"
)

type LeaderboardUseCase struct {
	repo leaderboardrepo.Repository
}

func NewLeaderboardUseCase(repo leaderboardrepo.Repository) *LeaderboardUseCase {
	return &LeaderboardUseCase{repo: repo}
}

func (uc *LeaderboardUseCase) GetExamSetLeaderboard(ctx context.Context, userID uuid.UUID, examSetCode string, filter domain.ListFilter) (*domain.ExamSetLeaderboardResponse, error) {
	set, err := uc.repo.FindPublishedExamSetByCode(ctx, examSetCode)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotPublished
	}

	page, limit, offset := normalizePagination(filter)

	total, err := uc.repo.CountExamSetLeaderboard(ctx, set.ID)
	if err != nil {
		return nil, err
	}

	rows, err := uc.repo.ListExamSetLeaderboard(ctx, set.ID, offset, limit)
	if err != nil {
		return nil, err
	}

	userRank, err := uc.repo.GetExamSetUserRank(ctx, set.ID, userID)
	if err != nil {
		return nil, err
	}

	entries := make([]domain.ExamSetLeaderboardEntry, len(rows))
	for i, row := range rows {
		duration := 0
		if row.DurationSeconds != nil {
			duration = *row.DurationSeconds
		}
		displayName := userdomain.PublicDisplayName(row.DisplayName, row.Email)
		entries[i] = domain.ExamSetLeaderboardEntry{
			Rank:            row.Rank,
			UserID:          row.UserID.String(),
			DisplayName:     displayName,
			IsCurrentUser:   row.UserID == userID,
			Score:           row.Score,
			TotalScore:      row.TotalScore,
			ScorePercent:    round1(row.ScorePercent),
			Passed:          int(row.ScorePercent) >= row.PassingScore,
			DurationSeconds: duration,
			SubmittedAt:     row.SubmittedAt,
		}
	}

	resp := &domain.ExamSetLeaderboardResponse{
		ExamSet: domain.ExamSetRef{
			Code:          set.Code,
			Title:         set.Title,
			ExamTrackName: set.ExamTrackName,
		},
		Leaderboard: entries,
		Pagination: domain.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}

	if userRank != nil {
		duration := 0
		if userRank.DurationSeconds != nil {
			duration = *userRank.DurationSeconds
		}
		resp.CurrentUserRank = &domain.ExamSetCurrentUserRank{
			Rank:            userRank.Rank,
			ScorePercent:    round1(userRank.ScorePercent),
			DurationSeconds: duration,
			SubmittedAt:     userRank.SubmittedAt,
		}
	}

	return resp, nil
}

func (uc *LeaderboardUseCase) GetExamTrackLeaderboard(ctx context.Context, userID uuid.UUID, trackCode string, filter domain.ListFilter) (*domain.ExamTrackLeaderboardResponse, error) {
	track, err := uc.repo.FindActiveExamTrackByCode(ctx, trackCode)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, apperrors.ErrExamTrackNotFound
	}

	page, limit, offset := normalizePagination(filter)

	total, err := uc.repo.CountExamTrackLeaderboard(ctx, track.ID)
	if err != nil {
		return nil, err
	}

	rows, err := uc.repo.ListExamTrackLeaderboard(ctx, track.ID, offset, limit)
	if err != nil {
		return nil, err
	}

	userRank, err := uc.repo.GetExamTrackUserRank(ctx, track.ID, userID)
	if err != nil {
		return nil, err
	}

	entries := make([]domain.ExamTrackLeaderboardEntry, len(rows))
	for i, row := range rows {
		displayName := userdomain.PublicDisplayName(row.DisplayName, row.Email)
		entries[i] = domain.ExamTrackLeaderboardEntry{
			Rank:                row.Rank,
			UserID:              row.UserID.String(),
			DisplayName:         displayName,
			IsCurrentUser:       row.UserID == userID,
			AverageScorePercent: round1(row.AverageScorePercent),
			CompletedExamSets:   row.CompletedExamSets,
			PassedExamSets:      row.PassedExamSets,
			PassRatePercent:     round1(row.PassRatePercent),
			LatestSubmittedAt:   row.LatestSubmittedAt,
		}
	}

	resp := &domain.ExamTrackLeaderboardResponse{
		ExamTrack: domain.ExamTrackRef{
			Code: track.Code,
			Name: track.Name,
		},
		Leaderboard: entries,
		Pagination: domain.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}

	if userRank != nil {
		resp.CurrentUserRank = &domain.ExamTrackCurrentUserRank{
			Rank:                userRank.Rank,
			AverageScorePercent: round1(userRank.AverageScorePercent),
			CompletedExamSets:   userRank.CompletedExamSets,
			PassedExamSets:      userRank.PassedExamSets,
			PassRatePercent:     round1(userRank.PassRatePercent),
		}
	}

	return resp, nil
}

func normalizePagination(filter domain.ListFilter) (page, limit, offset int) {
	page = filter.Page
	if page < 1 {
		page = 1
	}
	limit = filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	offset = (page - 1) * limit
	return page, limit, offset
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
