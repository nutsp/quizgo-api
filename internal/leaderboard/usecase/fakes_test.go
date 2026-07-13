package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

type projectionFixture struct {
	seasonID  uuid.UUID
	trackID   uuid.UUID
	examSetID uuid.UUID
	userID    uuid.UUID
	joinedAt  time.Time
	stoppedAt *time.Time

	bestScores map[uuid.UUID]domain.ScoreCandidate
	entries    map[uuid.UUID]leaderboardrepo.EntryRow

	scoreWrites int
	ensureCalls int
	joinCalls   int
	stopCalls   int
	joinedWith  time.Time
	stoppedWith time.Time
}

func newProjectionFixture() *projectionFixture {
	stoppedAt := at(20)
	return &projectionFixture{
		seasonID:   uuid.MustParse("10000000-0000-0000-0000-000000000001"),
		trackID:    uuid.MustParse("20000000-0000-0000-0000-000000000002"),
		examSetID:  uuid.MustParse("30000000-0000-0000-0000-000000000003"),
		userID:     uuid.MustParse("40000000-0000-0000-0000-000000000004"),
		joinedAt:   at(9),
		stoppedAt:  &stoppedAt,
		bestScores: make(map[uuid.UUID]domain.ScoreCandidate),
		entries:    make(map[uuid.UUID]leaderboardrepo.EntryRow),
	}
}

func (f *projectionFixture) EnsureSeason(_ context.Context, trackID uuid.UUID, window domain.SeasonWindow) (*leaderboardrepo.SeasonRow, error) {
	f.ensureCalls++
	if trackID != f.trackID {
		return nil, errors.New("unexpected track ID")
	}
	return &leaderboardrepo.SeasonRow{
		ID:          f.seasonID,
		ExamTrackID: f.trackID,
		Year:        window.Year,
		Month:       window.Month,
		StartsAt:    window.StartsAt,
		EndsAt:      window.EndsAt,
	}, nil
}

func (f *projectionFixture) JoinExamSet(_ context.Context, seasonID, examSetID uuid.UUID, joinedAt time.Time) error {
	f.joinCalls++
	if seasonID != f.seasonID || examSetID != f.examSetID {
		return errors.New("unexpected season or exam set ID")
	}
	f.joinedWith = joinedAt
	return nil
}

func (f *projectionFixture) StopExamSet(_ context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	f.stopCalls++
	if examSetID != f.examSetID {
		return errors.New("unexpected exam set ID")
	}
	f.stoppedWith = stoppedAt
	return nil
}

func (f *projectionFixture) GetEligibleSeason(_ context.Context, examSetID uuid.UUID, submittedAt time.Time) (*leaderboardrepo.SeasonRow, error) {
	if examSetID != f.examSetID {
		return nil, errors.New("unexpected exam set ID")
	}
	if submittedAt.Before(f.joinedAt) || (f.stoppedAt != nil && !submittedAt.Before(*f.stoppedAt)) {
		return nil, nil
	}
	return &leaderboardrepo.SeasonRow{
		ID:          f.seasonID,
		ExamTrackID: f.trackID,
		Year:        2026,
		Month:       7,
		StartsAt:    at(1),
		EndsAt:      at(32),
	}, nil
}

func (f *projectionFixture) UpsertBestScore(
	_ context.Context,
	seasonID, userID, examSetID, _ uuid.UUID,
	candidate domain.ScoreCandidate,
) (*leaderboardrepo.BestScoreUpdate, error) {
	if seasonID != f.seasonID || userID != f.userID || examSetID != f.examSetID {
		return nil, errors.New("unexpected score key")
	}

	current, exists := f.bestScores[examSetID]
	if exists && !domain.AttemptWins(candidate, current) {
		currentCopy := current
		return &leaderboardrepo.BestScoreUpdate{
			Previous: &currentCopy,
			Current:  current,
		}, nil
	}

	var previous *domain.ScoreCandidate
	if exists {
		currentCopy := current
		previous = &currentCopy
	}
	f.bestScores[examSetID] = candidate
	f.scoreWrites++
	return &leaderboardrepo.BestScoreUpdate{
		Previous: previous,
		Current:  candidate,
		Improved: true,
	}, nil
}

func (f *projectionFixture) RebuildEntry(_ context.Context, seasonID, userID uuid.UUID) (*leaderboardrepo.EntryRow, error) {
	if seasonID != f.seasonID || userID != f.userID {
		return nil, errors.New("unexpected entry key")
	}

	entry := leaderboardrepo.EntryRow{}
	for _, best := range f.bestScores {
		entry.TotalPoints += best.Points
		entry.CompletedExamSets++
		entry.TotalDurationSeconds += int64(best.DurationSeconds)
		if best.AchievedAt.After(entry.ScoreAchievedAt) {
			entry.ScoreAchievedAt = best.AchievedAt
		}
	}
	f.entries[userID] = entry
	return &entry, nil
}

func (f *projectionFixture) GetUserRank(_ context.Context, seasonID, userID uuid.UUID) (*leaderboardrepo.UserRankRow, error) {
	if seasonID != f.seasonID || userID != f.userID {
		return nil, errors.New("unexpected rank key")
	}
	if _, exists := f.entries[userID]; !exists {
		return nil, nil
	}

	userIDs := make([]uuid.UUID, 0, len(f.entries))
	for entryUserID := range f.entries {
		userIDs = append(userIDs, entryUserID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i].String() < userIDs[j].String() })
	for index, entryUserID := range userIDs {
		if entryUserID == userID {
			return &leaderboardrepo.UserRankRow{
				Rank:        index + 1,
				TotalPoints: f.entries[userID].TotalPoints,
			}, nil
		}
	}
	return nil, nil
}

func (f *projectionFixture) RecordProjectionFailure(context.Context, uuid.UUID, error) error {
	return nil
}

func at(day int) time.Time {
	return time.Date(2026, time.July, day, 10, 0, 0, 0, time.UTC)
}

func score(points float64, duration int, achievedAt time.Time) *domain.ScoreCandidate {
	value := candidate(points, duration, achievedAt)
	return &value
}

func candidate(points float64, duration int, achievedAt time.Time) domain.ScoreCandidate {
	return domain.ScoreCandidate{
		Points:          points,
		DurationSeconds: duration,
		AchievedAt:      achievedAt,
	}
}
