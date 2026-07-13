package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

func TestProjectAttemptKeepsOnlyEligibleBestScore(t *testing.T) {
	tests := []struct {
		name            string
		existing        *domain.ScoreCandidate
		submittedAt     time.Time
		candidate       domain.ScoreCandidate
		wantPoints      float64
		wantPointsAdded float64
		wantImproved    bool
		wantScoreWrites int
	}{
		{"keeps higher existing score", score(82, 900, at(10)), at(12), candidate(75, 700, at(12)), 82, 0, false, 0},
		{"replaces tie with shorter duration", score(82, 900, at(10)), at(12), candidate(82, 800, at(12)), 82, 0, true, 1},
		{"rejects before join", nil, at(8), candidate(90, 800, at(8)), 0, 0, false, 0},
		{"rejects at stopped time", nil, at(20), candidate(90, 800, at(20)), 0, 0, false, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProjectionFixture()
			if test.existing != nil {
				fixture.bestScores[fixture.examSetID] = *test.existing
				fixture.entries[fixture.userID] = entryFor(*test.existing)
			}
			projector := NewProjector(fixture)

			got, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
				AttemptID:   uuid.New(),
				UserID:      fixture.userID,
				ExamSetID:   fixture.examSetID,
				ExamTrackID: fixture.trackID,
				TrackCode:   "math",
				SubmittedAt: test.submittedAt,
				Candidate:   test.candidate,
			})
			if err != nil {
				t.Fatalf("ProjectAttempt() error = %v", err)
			}
			if got.TotalPoints != test.wantPoints {
				t.Errorf("TotalPoints = %.1f, want %.1f", got.TotalPoints, test.wantPoints)
			}
			if got.PointsAdded != test.wantPointsAdded {
				t.Errorf("PointsAdded = %.1f, want %.1f", got.PointsAdded, test.wantPointsAdded)
			}
			if got.ImprovedBestScore != test.wantImproved {
				t.Errorf("ImprovedBestScore = %t, want %t", got.ImprovedBestScore, test.wantImproved)
			}
			if fixture.scoreWrites != test.wantScoreWrites {
				t.Errorf("score writes = %d, want %d", fixture.scoreWrites, test.wantScoreWrites)
			}
		})
	}
}

func TestProjectAttemptRetryIsIdempotent(t *testing.T) {
	fixture := newProjectionFixture()
	projector := NewProjector(fixture)
	input := domain.ProjectionInput{
		AttemptID:   uuid.MustParse("50000000-0000-0000-0000-000000000005"),
		UserID:      fixture.userID,
		ExamSetID:   fixture.examSetID,
		ExamTrackID: fixture.trackID,
		TrackCode:   "math",
		SubmittedAt: at(12),
		Candidate:   candidate(90, 800, at(12)),
	}

	first, err := projector.ProjectAttempt(context.Background(), input)
	if err != nil {
		t.Fatalf("first ProjectAttempt() error = %v", err)
	}
	second, err := projector.ProjectAttempt(context.Background(), input)
	if err != nil {
		t.Fatalf("second ProjectAttempt() error = %v", err)
	}

	if len(fixture.bestScores) != 1 {
		t.Errorf("score rows = %d, want 1", len(fixture.bestScores))
	}
	if len(fixture.entries) != 1 {
		t.Errorf("entry rows = %d, want 1", len(fixture.entries))
	}
	if fixture.scoreWrites != 1 {
		t.Errorf("score writes = %d, want 1", fixture.scoreWrites)
	}
	if first.TotalPoints != second.TotalPoints {
		t.Errorf("retry total = %.1f, want %.1f", second.TotalPoints, first.TotalPoints)
	}
	if first.BestScoreAfter != second.BestScoreAfter {
		t.Errorf("retry best score = %.1f, want %.1f", second.BestScoreAfter, first.BestScoreAfter)
	}
}

func TestProjectAttemptRetryRepairsMissingAggregate(t *testing.T) {
	fixture := newProjectionFixture()
	fixture.bestScores[fixture.examSetID] = candidate(90, 800, at(12))
	projector := NewProjector(fixture)

	got, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
		AttemptID:   uuid.MustParse("50000000-0000-0000-0000-000000000005"),
		UserID:      fixture.userID,
		ExamSetID:   fixture.examSetID,
		ExamTrackID: fixture.trackID,
		TrackCode:   "math",
		SubmittedAt: at(12),
		Candidate:   candidate(90, 800, at(12)),
	})
	if err != nil {
		t.Fatalf("ProjectAttempt() error = %v", err)
	}

	if len(fixture.entries) != 1 {
		t.Errorf("entry rows = %d, want 1", len(fixture.entries))
	}
	if got.TotalPoints != 90 {
		t.Errorf("TotalPoints = %.1f, want 90", got.TotalPoints)
	}
	if got.ImprovedBestScore {
		t.Error("ImprovedBestScore = true, want false for an identical retry")
	}
}

func TestProjectAttemptClampsAndRoundsPoints(t *testing.T) {
	tests := []struct {
		name   string
		points float64
		want   float64
	}{
		{"clamps below zero", -0.04, 0},
		{"rounds to one decimal", 82.26, 82.3},
		{"clamps above one hundred", 100.06, 100},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProjectionFixture()
			projector := NewProjector(fixture)

			got, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
				AttemptID:   uuid.New(),
				UserID:      fixture.userID,
				ExamSetID:   fixture.examSetID,
				ExamTrackID: fixture.trackID,
				TrackCode:   "math",
				SubmittedAt: at(12),
				Candidate:   candidate(test.points, 800, at(12)),
			})
			if err != nil {
				t.Fatalf("ProjectAttempt() error = %v", err)
			}
			if got.BestScoreAfter != test.want {
				t.Errorf("BestScoreAfter = %.1f, want %.1f", got.BestScoreAfter, test.want)
			}
		})
	}
}

func TestProjectorManagesSeasonEnrollmentLifecycle(t *testing.T) {
	fixture := newProjectionFixture()
	projector := NewProjector(fixture)
	publishedAt := time.Date(2026, time.June, 30, 17, 30, 0, 0, time.UTC)

	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, publishedAt); err != nil {
		t.Fatalf("OnExamSetPublished() error = %v", err)
	}
	if fixture.ensureCalls != 1 || fixture.joinCalls != 1 {
		t.Fatalf("ensure/join calls = %d/%d, want 1/1", fixture.ensureCalls, fixture.joinCalls)
	}
	if !fixture.joinedWith.Equal(publishedAt) {
		t.Errorf("joined at = %s, want %s", fixture.joinedWith, publishedAt)
	}

	stoppedAt := publishedAt.Add(time.Hour)
	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, stoppedAt); err != nil {
		t.Fatalf("OnExamSetStopped() error = %v", err)
	}
	if fixture.stopCalls != 1 {
		t.Errorf("stop calls = %d, want 1", fixture.stopCalls)
	}
	if !fixture.stoppedWith.Equal(stoppedAt) {
		t.Errorf("stopped at = %s, want %s", fixture.stoppedWith, stoppedAt)
	}
}

func entryFor(best domain.ScoreCandidate) leaderboardrepo.EntryRow {
	return leaderboardrepo.EntryRow{
		TotalPoints:          best.Points,
		CompletedExamSets:    1,
		TotalDurationSeconds: int64(best.DurationSeconds),
		ScoreAchievedAt:      best.AchievedAt,
	}
}
