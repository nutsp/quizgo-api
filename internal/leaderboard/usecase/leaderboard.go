package usecase

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	userdomain "virtual-exam-api/internal/user/domain"
)

type LeaderboardRepository interface {
	FindPublishedExamSetByCode(context.Context, string) (*leaderboardrepo.ExamSetContextRow, error)
	FindActiveExamTrackByCode(context.Context, string) (*leaderboardrepo.ExamTrackContextRow, error)
	CountExamSetLeaderboard(context.Context, uuid.UUID) (int64, error)
	ListExamSetLeaderboard(context.Context, uuid.UUID, int, int) ([]leaderboardrepo.ExamSetLeaderboardRow, error)
	GetExamSetUserRank(context.Context, uuid.UUID, uuid.UUID) (*leaderboardrepo.ExamSetUserRankRow, error)
	CountExamTrackLeaderboard(context.Context, uuid.UUID) (int64, error)
	ListExamTrackLeaderboard(context.Context, uuid.UUID, int, int) ([]leaderboardrepo.ExamTrackLeaderboardRow, error)
	GetExamTrackUserRank(context.Context, uuid.UUID, uuid.UUID) (*leaderboardrepo.ExamTrackUserRankRow, error)
	EnsureSeason(context.Context, uuid.UUID, domain.SeasonWindow) (*leaderboardrepo.SeasonRow, error)
	FindMostRecentAttemptedTrack(context.Context, uuid.UUID) (*leaderboardrepo.ExamTrackContextRow, error)
	FindSeason(context.Context, uuid.UUID, int, int) (*leaderboardrepo.SeasonRow, error)
	CountSeasonLeaderboard(context.Context, uuid.UUID) (int64, error)
	ListSeasonLeaderboard(context.Context, uuid.UUID, int, int) ([]leaderboardrepo.SeasonLeaderboardRow, error)
	ListSeasonTopThree(context.Context, uuid.UUID) ([]leaderboardrepo.SeasonLeaderboardRow, error)
	ListSeasonLeaderboardAroundUser(context.Context, uuid.UUID, uuid.UUID, int, int) ([]leaderboardrepo.SeasonLeaderboardRow, error)
	GetSeasonUserSummary(context.Context, uuid.UUID, uuid.UUID) (*leaderboardrepo.SeasonUserSummaryRow, error)
	ListNextOpportunities(context.Context, uuid.UUID, uuid.UUID) ([]leaderboardrepo.NextOpportunityRow, error)
	ListAwards(context.Context, uuid.UUID) ([]leaderboardrepo.AwardRow, error)
	ListDueSeasons(context.Context, time.Time) ([]leaderboardrepo.SeasonRow, error)
	FinalizeSeason(context.Context, uuid.UUID, time.Time) (*leaderboardrepo.FinalizationResult, error)
	ReconcileSeason(context.Context, uuid.UUID, domain.SeasonWindow) (*leaderboardrepo.ReconcileResult, error)
}

type LeaderboardUseCase struct {
	repo   LeaderboardRepository
	now    func() time.Time
	logger *log.Logger
}

func NewLeaderboardUseCase(repo LeaderboardRepository) *LeaderboardUseCase {
	return &LeaderboardUseCase{repo: repo, now: time.Now, logger: log.Default()}
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
