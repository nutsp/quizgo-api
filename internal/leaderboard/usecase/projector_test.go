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
	fixture.intervals = nil
	projector := NewProjector(fixture)
	publishedAt := at(9)

	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, publishedAt); err != nil {
		t.Fatalf("OnExamSetPublished() error = %v", err)
	}
	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, at(10)); err != nil {
		t.Fatalf("repeated OnExamSetPublished() error = %v", err)
	}
	if len(fixture.intervals) != 1 {
		t.Fatalf("intervals after repeated publish = %d, want 1", len(fixture.intervals))
	}
	if !fixture.intervals[0].joinedAt.Equal(publishedAt) || fixture.intervals[0].stoppedAt != nil {
		t.Errorf("first interval = %+v, want open interval joined at %s", fixture.intervals[0], publishedAt)
	}

	first, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
		AttemptID:   uuid.New(),
		UserID:      fixture.userID,
		ExamSetID:   fixture.examSetID,
		ExamTrackID: fixture.trackID,
		TrackCode:   "math",
		SubmittedAt: at(11),
		Candidate:   candidate(80, 900, at(11)),
	})
	if err != nil {
		t.Fatalf("first ProjectAttempt() error = %v", err)
	}
	if first.TotalPoints != 80 {
		t.Errorf("first TotalPoints = %.1f, want 80", first.TotalPoints)
	}

	stoppedAt := at(12)
	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, stoppedAt); err != nil {
		t.Fatalf("OnExamSetStopped() error = %v", err)
	}
	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, at(13)); err != nil {
		t.Fatalf("repeated OnExamSetStopped() error = %v", err)
	}
	if fixture.intervals[0].stoppedAt == nil || !fixture.intervals[0].stoppedAt.Equal(stoppedAt) {
		t.Errorf("first stopped at = %v, want %s", fixture.intervals[0].stoppedAt, stoppedAt)
	}

	gap, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
		AttemptID:   uuid.New(),
		UserID:      fixture.userID,
		ExamSetID:   fixture.examSetID,
		ExamTrackID: fixture.trackID,
		TrackCode:   "math",
		SubmittedAt: at(14),
		Candidate:   candidate(95, 700, at(14)),
	})
	if err != nil {
		t.Fatalf("gap ProjectAttempt() error = %v", err)
	}
	if gap.SeasonID != "" || fixture.bestScores[fixture.examSetID].Points != 80 {
		t.Errorf("gap attempt changed score: season = %q, best = %.1f", gap.SeasonID, fixture.bestScores[fixture.examSetID].Points)
	}

	republishedAt := at(15)
	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, republishedAt); err != nil {
		t.Fatalf("republished OnExamSetPublished() error = %v", err)
	}
	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, at(16)); err != nil {
		t.Fatalf("repeated republished OnExamSetPublished() error = %v", err)
	}
	if len(fixture.intervals) != 2 {
		t.Fatalf("intervals after republish = %d, want 2", len(fixture.intervals))
	}
	if !fixture.intervals[1].joinedAt.Equal(republishedAt) || fixture.intervals[1].stoppedAt != nil {
		t.Errorf("second interval = %+v, want open interval joined at %s", fixture.intervals[1], republishedAt)
	}

	afterRepublish, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
		AttemptID:   uuid.New(),
		UserID:      fixture.userID,
		ExamSetID:   fixture.examSetID,
		ExamTrackID: fixture.trackID,
		TrackCode:   "math",
		SubmittedAt: republishedAt,
		Candidate:   candidate(90, 800, republishedAt),
	})
	if err != nil {
		t.Fatalf("republished ProjectAttempt() error = %v", err)
	}
	if afterRepublish.TotalPoints != 90 || len(fixture.bestScores) != 1 {
		t.Errorf("republished score total/rows = %.1f/%d, want 90/1", afterRepublish.TotalPoints, len(fixture.bestScores))
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
