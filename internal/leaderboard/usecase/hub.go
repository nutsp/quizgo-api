package usecase

import (
	"context"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	userdomain "virtual-exam-api/internal/user/domain"
)

const (
	ScopeTop      = "top"
	ScopeAroundMe = "around_me"
)

func (uc *LeaderboardUseCase) GetOverview(ctx context.Context, userID uuid.UUID) (*domain.OverviewResponse, error) {
	response := &domain.OverviewResponse{
		TopThree:          []domain.LeaderboardEntry{},
		NextOpportunities: []domain.ExamSetRef{},
	}
	track, err := uc.repo.FindMostRecentAttemptedTrack(ctx, userID)
	if err != nil {
		return nil, err
	}
	if track == nil {
		if _, err := uc.FinalizeDueSeasons(ctx, uc.now().UTC()); err != nil {
			return nil, err
		}
		return response, nil
	}

	hub, err := uc.GetHub(ctx, userID, track.Code, 0, 0, ScopeTop, domain.ListFilter{})
	if err != nil {
		return nil, err
	}
	trackCode := track.Code
	response.DefaultTrackCode = &trackCode
	response.Season = &hub.Season
	response.ExamTrack = &hub.ExamTrack
	response.CurrentUser = hub.CurrentUser
	response.TopThree = hub.TopThree
	response.NextOpportunities = hub.NextOpportunities
	return response, nil
}

func (uc *LeaderboardUseCase) GetHub(
	ctx context.Context,
	userID uuid.UUID,
	trackCode string,
	year, month int,
	scope string,
	filter domain.ListFilter,
) (*domain.HubResponse, error) {
	now := uc.now().UTC()
	if _, err := uc.FinalizeDueSeasons(ctx, now); err != nil {
		return nil, err
	}

	track, err := uc.repo.FindActiveExamTrackByCode(ctx, trackCode)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, apperrors.ErrExamTrackNotFound
	}

	currentWindow, err := domain.BangkokSeasonWindow(now)
	if err != nil {
		return nil, err
	}
	if year == 0 && month == 0 {
		year, month = currentWindow.Year, currentWindow.Month
	}
	if year == 0 || month == 0 {
		return nil, apperrors.ValidationError("year and month must be provided together")
	}

	var season *leaderboardrepo.SeasonRow
	if year == currentWindow.Year && month == currentWindow.Month {
		season, err = uc.repo.EnsureSeason(ctx, track.ID, currentWindow)
	} else {
		season, err = uc.repo.FindSeason(ctx, track.ID, year, month)
	}
	if err != nil {
		return nil, err
	}
	if season == nil {
		return nil, apperrors.ErrNotFound
	}

	total, err := uc.repo.CountSeasonLeaderboard(ctx, season.ID)
	if err != nil {
		return nil, err
	}
	topRows, err := uc.repo.ListSeasonTopThree(ctx, season.ID)
	if err != nil {
		return nil, err
	}
	userSummary, err := uc.repo.GetSeasonUserSummary(ctx, season.ID, userID)
	if err != nil {
		return nil, err
	}

	page, limit, offset := normalizeHubPagination(filter)
	var rows []leaderboardrepo.SeasonLeaderboardRow
	switch scope {
	case "", ScopeTop:
		rows, err = uc.repo.ListSeasonLeaderboard(ctx, season.ID, offset, limit)
	case ScopeAroundMe:
		rows, err = uc.repo.ListSeasonLeaderboardAroundUser(ctx, season.ID, userID, 5, 5)
		page = 1
		limit = 11
	default:
		return nil, apperrors.ValidationError("scope must be top or around_me")
	}
	if err != nil {
		return nil, err
	}

	opportunities := []leaderboardrepo.NextOpportunityRow{}
	if season.Status != "finalized" {
		opportunities, err = uc.repo.ListNextOpportunities(ctx, season.ID, userID)
		if err != nil {
			return nil, err
		}
	}

	response := &domain.HubResponse{
		Season: domain.SeasonWindow{
			Year: season.Year, Month: season.Month, StartsAt: season.StartsAt, EndsAt: season.EndsAt,
			Status: season.Status, FinalizedAt: season.FinalizedAt,
		},
		ExamTrack:         domain.ExamTrackRef{Code: track.Code, Name: track.Name},
		CurrentUser:       mapCurrentUserSummary(userSummary),
		TopThree:          mapSeasonEntries(topRows, userID),
		Leaderboard:       mapSeasonEntries(rows, userID),
		NextOpportunities: mapOpportunities(opportunities),
		Pagination:        domain.Pagination{Page: page, Limit: limit, Total: total},
	}
	return response, nil
}

func (uc *LeaderboardUseCase) ListAwards(ctx context.Context, userID uuid.UUID) ([]domain.Award, error) {
	if _, err := uc.FinalizeDueSeasons(ctx, uc.now().UTC()); err != nil {
		return nil, err
	}
	rows, err := uc.repo.ListAwards(ctx, userID)
	if err != nil {
		return nil, err
	}
	awards := make([]domain.Award, len(rows))
	for index, row := range rows {
		awards[index] = domain.Award{
			SeasonID: row.SeasonID.String(), TrackCode: row.TrackCode,
			Year: row.Year, Month: row.Month, Rank: row.Rank,
			Medal: medalForRank(row.Rank), AwardedAt: row.AwardedAt,
		}
	}
	return awards, nil
}

func normalizeHubPagination(filter domain.ListFilter) (page, limit, offset int) {
	page = filter.Page
	if page < 1 {
		page = 1
	}
	limit = filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit, (page - 1) * limit
}

func mapSeasonEntries(rows []leaderboardrepo.SeasonLeaderboardRow, currentUserID uuid.UUID) []domain.LeaderboardEntry {
	entries := make([]domain.LeaderboardEntry, len(rows))
	for index, row := range rows {
		entries[index] = domain.LeaderboardEntry{
			Rank: row.Rank, UserID: row.UserID.String(),
			DisplayName:   userdomain.PublicDisplayName(row.DisplayName, row.Email),
			IsCurrentUser: row.UserID == currentUserID, TotalPoints: round1(row.TotalPoints),
			CompletedExamSets: row.CompletedExamSets, TotalDurationSeconds: row.TotalDurationSeconds,
			ScoreAchievedAt: row.ScoreAchievedAt,
		}
	}
	return entries
}

func mapCurrentUserSummary(row *leaderboardrepo.SeasonUserSummaryRow) *domain.CurrentUserSummary {
	if row == nil {
		return nil
	}
	rank := row.Rank
	return &domain.CurrentUserSummary{
		Rank: &rank, TotalPoints: round1(row.TotalPoints), CompletedExamSets: row.CompletedExamSets,
		TotalDurationSeconds: row.TotalDurationSeconds, ScoreAchievedAt: row.ScoreAchievedAt,
	}
}

func mapOpportunities(rows []leaderboardrepo.NextOpportunityRow) []domain.ExamSetRef {
	opportunities := make([]domain.ExamSetRef, len(rows))
	for index, row := range rows {
		opportunities[index] = domain.ExamSetRef{Code: row.Code, Title: row.Title, ExamTrackName: row.ExamTrackName}
	}
	return opportunities
}

func medalForRank(rank int) string {
	switch rank {
	case 1:
		return "gold"
	case 2:
		return "silver"
	case 3:
		return "bronze"
	default:
		return ""
	}
}
