package usecase

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

type ProjectorRepository interface {
	EnsureSeason(context.Context, uuid.UUID, domain.SeasonWindow) (*leaderboardrepo.SeasonRow, error)
	JoinExamSet(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	StopExamSet(context.Context, uuid.UUID, time.Time) error
	GetEligibleSeason(context.Context, uuid.UUID, time.Time) (*leaderboardrepo.SeasonRow, error)
	UpsertBestScore(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, domain.ScoreCandidate) (*leaderboardrepo.BestScoreUpdate, error)
	RebuildEntry(context.Context, uuid.UUID, uuid.UUID) (*leaderboardrepo.EntryRow, error)
	GetUserRank(context.Context, uuid.UUID, uuid.UUID) (*leaderboardrepo.UserRankRow, error)
	RecordProjectionFailure(context.Context, uuid.UUID, error) error
}

type Projector struct {
	repo ProjectorRepository
}

func NewProjector(repo ProjectorRepository) *Projector {
	return &Projector{repo: repo}
}

func (p *Projector) ProjectAttempt(ctx context.Context, input domain.ProjectionInput) (*domain.ProjectionUpdate, error) {
	input.Candidate.Points = normalizeProjectedPoints(input.Candidate.Points)
	result := &domain.ProjectionUpdate{TrackCode: input.TrackCode}

	season, err := p.repo.GetEligibleSeason(ctx, input.ExamSetID, input.SubmittedAt)
	if err != nil {
		return nil, err
	}
	if season == nil {
		return result, nil
	}
	result.SeasonID = season.ID.String()
	result.Year = season.Year
	result.Month = season.Month

	previousRank, err := p.repo.GetUserRank(ctx, season.ID, input.UserID)
	if err != nil {
		return nil, err
	}

	scoreUpdate, err := p.repo.UpsertBestScore(
		ctx,
		season.ID,
		input.UserID,
		input.ExamSetID,
		input.AttemptID,
		input.Candidate,
	)
	if err != nil {
		return nil, err
	}
	if scoreUpdate.Previous != nil {
		result.BestScoreBefore = scoreUpdate.Previous.Points
	}
	result.BestScoreAfter = scoreUpdate.Current.Points
	result.ImprovedBestScore = scoreUpdate.Improved
	result.PointsAdded = normalizeProjectedPoints(scoreUpdate.Current.Points - result.BestScoreBefore)

	entry, err := p.repo.RebuildEntry(ctx, season.ID, input.UserID)
	if err != nil {
		return nil, err
	}
	result.TotalPoints = entry.TotalPoints

	currentRank, err := p.repo.GetUserRank(ctx, season.ID, input.UserID)
	if err != nil {
		return nil, err
	}
	if previousRank != nil {
		rank := previousRank.Rank
		result.PreviousRank = &rank
	}
	if currentRank != nil {
		result.CurrentRank = currentRank.Rank
	}
	return result, nil
}

func (p *Projector) OnExamSetPublished(ctx context.Context, examTrackID, examSetID uuid.UUID, publishedAt time.Time) error {
	window, err := domain.BangkokSeasonWindow(publishedAt)
	if err != nil {
		return err
	}
	season, err := p.repo.EnsureSeason(ctx, examTrackID, window)
	if err != nil {
		return err
	}
	return p.repo.JoinExamSet(ctx, season.ID, examSetID, publishedAt)
}

func (p *Projector) OnExamSetStopped(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	return p.repo.StopExamSet(ctx, examSetID, stoppedAt)
}

func (p *Projector) RecordProjectionFailure(ctx context.Context, attemptID uuid.UUID, projectionErr error) error {
	return p.repo.RecordProjectionFailure(ctx, attemptID, projectionErr)
}

func normalizeProjectedPoints(points float64) float64 {
	points = max(0, min(100, points))
	return math.Round(points*10) / 10
}
