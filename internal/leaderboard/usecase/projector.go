package usecase

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

type ProjectorRepository interface {
	EnsureSeason(context.Context, uuid.UUID, domain.SeasonWindow) (*leaderboardrepo.SeasonRow, error)
	EnsureSeasonForPublish(context.Context, uuid.UUID, uuid.UUID, domain.SeasonWindow) (*leaderboardrepo.SeasonRow, error)
	PublishExamSet(context.Context, uuid.UUID, uuid.UUID, domain.SeasonWindow, time.Time) (*leaderboardrepo.SeasonRow, error)
	JoinExamSet(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	StopExamSet(context.Context, uuid.UUID, time.Time) error
	GetEligibleSeason(context.Context, uuid.UUID, time.Time) (*leaderboardrepo.SeasonRow, error)
	ProjectBestScore(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time, domain.ScoreCandidate) (*leaderboardrepo.BestScoreProjection, error)
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
	if err := validateProjectionCandidate(input.Candidate); err != nil {
		return nil, err
	}
	input.Candidate.Points = normalizeProjectedPoints(input.Candidate.Points)
	result := &domain.ProjectionUpdate{TrackCode: input.TrackCode}

	window, err := domain.BangkokSeasonWindow(input.SubmittedAt)
	if err != nil {
		return nil, err
	}
	if _, err := p.repo.EnsureSeason(ctx, input.ExamTrackID, window); err != nil {
		return nil, err
	}

	projection, err := p.repo.ProjectBestScore(
		ctx,
		input.UserID,
		input.ExamSetID,
		input.AttemptID,
		input.SubmittedAt,
		input.Candidate,
	)
	if err != nil {
		return nil, err
	}
	if projection.Season == nil {
		return result, nil
	}
	season := projection.Season
	result.SeasonID = season.ID.String()
	result.Year = season.Year
	result.Month = season.Month

	scoreUpdate := projection.ScoreUpdate
	if scoreUpdate.Previous != nil {
		result.BestScoreBefore = scoreUpdate.Previous.Points
	}
	result.BestScoreAfter = scoreUpdate.Current.Points
	result.ImprovedBestScore = scoreUpdate.Improved
	result.PointsAdded = normalizeProjectedPoints(scoreUpdate.Current.Points - result.BestScoreBefore)
	result.TotalPoints = projection.Entry.TotalPoints

	if projection.PreviousRank != nil {
		rank := projection.PreviousRank.Rank
		result.PreviousRank = &rank
	}
	if projection.CurrentRank != nil {
		result.CurrentRank = projection.CurrentRank.Rank
	}
	return result, nil
}

// OnExamSetPublished consumes the activation event time persisted with the
// publication transition. Retries must reuse that time rather than time.Now.
func (p *Projector) OnExamSetPublished(ctx context.Context, examTrackID, examSetID uuid.UUID, publishedAt time.Time) error {
	window, err := domain.BangkokSeasonWindow(publishedAt)
	if err != nil {
		return err
	}
	_, err = p.repo.PublishExamSet(ctx, examTrackID, examSetID, window, publishedAt)
	return err
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

func validateProjectionCandidate(candidate domain.ScoreCandidate) error {
	if math.IsNaN(candidate.Points) || math.IsInf(candidate.Points, 0) {
		return errors.New("leaderboard projection points must be finite")
	}
	if candidate.DurationSeconds < 0 {
		return errors.New("leaderboard projection duration must not be negative")
	}
	return nil
}
