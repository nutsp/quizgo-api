package usecase

import (
	"context"
	"math"
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

func TestProjectAttemptRejectsInvalidCandidateValues(t *testing.T) {
	tests := []struct {
		name     string
		points   float64
		duration int
	}{
		{"not-a-number points", math.NaN(), 800},
		{"positive infinite points", math.Inf(1), 800},
		{"negative infinite points", math.Inf(-1), 800},
		{"negative duration", 80, -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newProjectionFixture()
			projector := NewProjector(fixture)

			_, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
				AttemptID:   uuid.New(),
				UserID:      fixture.userID,
				ExamSetID:   fixture.examSetID,
				ExamTrackID: fixture.trackID,
				TrackCode:   "math",
				SubmittedAt: at(12),
				Candidate:   candidate(test.points, test.duration, at(12)),
			})
			if err == nil {
				t.Fatal("ProjectAttempt() error = nil, want invalid candidate error")
			}
			if fixture.scoreWrites != 0 {
				t.Errorf("score writes = %d, want 0", fixture.scoreWrites)
			}
			if fixture.ensureCalls != 0 {
				t.Errorf("ensure calls = %d, want 0", fixture.ensureCalls)
			}
		})
	}
}

func TestProjectAttemptEnrollsActiveSetAtBangkokMonthRollover(t *testing.T) {
	fixture := newProjectionFixture()
	fixture.intervals = nil
	fixture.enrollActiveOnEnsure = true
	projector := NewProjector(fixture)
	submittedAt := at(12)

	got, err := projector.ProjectAttempt(context.Background(), domain.ProjectionInput{
		AttemptID:   uuid.New(),
		UserID:      fixture.userID,
		ExamSetID:   fixture.examSetID,
		ExamTrackID: fixture.trackID,
		TrackCode:   "math",
		SubmittedAt: submittedAt,
		Candidate:   candidate(88, 700, submittedAt),
	})
	if err != nil {
		t.Fatalf("ProjectAttempt() error = %v", err)
	}
	if got.TotalPoints != 88 {
		t.Errorf("TotalPoints = %.1f, want 88", got.TotalPoints)
	}
	if fixture.ensureCalls != 1 {
		t.Fatalf("ensure calls = %d, want 1", fixture.ensureCalls)
	}
	if len(fixture.intervals) != 1 {
		t.Fatalf("intervals = %d, want 1", len(fixture.intervals))
	}
	window, err := domain.BangkokSeasonWindow(submittedAt)
	if err != nil {
		t.Fatalf("BangkokSeasonWindow() error = %v", err)
	}
	if !fixture.intervals[0].joinedAt.Equal(window.StartsAt) {
		t.Errorf("joined at = %s, want season start %s", fixture.intervals[0].joinedAt, window.StartsAt)
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
	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, publishedAt); err != nil {
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
	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, stoppedAt); err != nil {
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
	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, republishedAt); err != nil {
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

func TestProjectorJoinsNewSetAtPublishTimeWhenCreatingSeason(t *testing.T) {
	fixture := newProjectionFixture()
	fixture.intervals = nil
	fixture.enrollActiveOnEnsure = true
	projector := NewProjector(fixture)
	publishedAt := at(15)

	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, publishedAt); err != nil {
		t.Fatalf("OnExamSetPublished() error = %v", err)
	}
	if len(fixture.intervals) != 1 {
		t.Fatalf("intervals = %d, want only the publish-time interval", len(fixture.intervals))
	}
	if !fixture.intervals[0].joinedAt.Equal(publishedAt) {
		t.Errorf("joined at = %s, want publish time %s", fixture.intervals[0].joinedAt, publishedAt)
	}
}

func TestProjectorLifecycleUsesEventTimeForStaleRetries(t *testing.T) {
	fixture := newProjectionFixture()
	fixture.intervals = []eligibilityInterval{{joinedAt: at(9)}}
	projector := NewProjector(fixture)
	republishedAt := at(15)

	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, republishedAt); err != nil {
		t.Fatalf("republish error = %v", err)
	}
	if len(fixture.intervals) != 2 {
		t.Fatalf("intervals after republish = %d, want 2", len(fixture.intervals))
	}
	if fixture.intervals[0].stoppedAt == nil || !fixture.intervals[0].stoppedAt.Equal(republishedAt) {
		t.Errorf("older interval stopped at = %v, want %s", fixture.intervals[0].stoppedAt, republishedAt)
	}
	if fixture.intervals[1].stoppedAt != nil || !fixture.intervals[1].joinedAt.Equal(republishedAt) {
		t.Errorf("new interval = %+v, want open at %s", fixture.intervals[1], republishedAt)
	}

	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, at(12)); err != nil {
		t.Fatalf("delayed stop error = %v", err)
	}
	if fixture.intervals[1].stoppedAt != nil {
		t.Errorf("delayed stop closed newer interval at %v", fixture.intervals[1].stoppedAt)
	}

	for _, retryAt := range []time.Time{republishedAt, at(10)} {
		if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, retryAt); err != nil {
			t.Fatalf("publish retry at %s error = %v", retryAt, err)
		}
	}
	if len(fixture.intervals) != 2 || fixture.intervals[1].stoppedAt != nil {
		t.Errorf("publish retries changed intervals = %+v", fixture.intervals)
	}

	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, at(20)); err != nil {
		t.Fatalf("stop error = %v", err)
	}
	if err := projector.OnExamSetStopped(context.Background(), fixture.examSetID, at(20)); err != nil {
		t.Fatalf("stop retry error = %v", err)
	}
	if fixture.intervals[1].stoppedAt == nil || !fixture.intervals[1].stoppedAt.Equal(at(20)) {
		t.Errorf("new interval stopped at = %v, want %s", fixture.intervals[1].stoppedAt, at(20))
	}

	if err := projector.OnExamSetPublished(context.Background(), fixture.trackID, fixture.examSetID, republishedAt); err != nil {
		t.Fatalf("historical publish retry error = %v", err)
	}
	if len(fixture.intervals) != 2 {
		t.Errorf("historical publish retry reopened interval count = %d, want 2", len(fixture.intervals))
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
