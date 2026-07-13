package repository_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardusecase "virtual-exam-api/internal/leaderboard/usecase"
)

const postgresRaceBound = 5 * time.Second

func TestPostgresInitialBootstrapUsesAuthoritativePublicationTime(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	publishedAt := window.StartsAt.Add(48 * time.Hour)
	unrelatedEditAt := publishedAt.Add(24 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, published_at, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?, ?)
	`, examSetID, trackID, publishedAt, window.StartsAt.Add(-time.Hour), unrelatedEditAt)

	season, err := repo.EnsureSeason(ctx, trackID, window)
	if err != nil {
		t.Fatalf("EnsureSeason() error = %v", err)
	}
	var joinedAt time.Time
	if err := db.Raw(`
		SELECT joined_at
		FROM leaderboard_season_exam_sets
		WHERE season_id = ? AND exam_set_id = ?
	`, season.ID, examSetID).Scan(&joinedAt).Error; err != nil {
		t.Fatalf("read bootstrap enrollment: %v", err)
	}
	if !joinedAt.Equal(publishedAt) {
		t.Errorf("bootstrap joined_at = %s, want %s", joinedAt, publishedAt)
	}
}

func TestPostgresApplicationReconcileBackfillsAndRestoresExclusion(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	trackID := uuid.New()
	examSetID := uuid.New()
	seasonID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	createdAt := window.StartsAt.Add(time.Hour)
	updatedAt := createdAt.Add(2 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, createdAt, updatedAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		ALTER TABLE leaderboard_season_exam_sets
		DROP CONSTRAINT leaderboard_season_exam_sets_no_overlap
	`)

	if err := examsetrepo.ReconcilePublicationState(db); err != nil {
		t.Fatalf("ReconcilePublicationState() error = %v", err)
	}
	if err := leaderboardrepo.ReconcileLifecycleSchema(db); err != nil {
		t.Fatalf("ReconcileLifecycleSchema() error = %v", err)
	}
	if got := readExamSetPublishedAt(t, db, examSetID); !got.Equal(updatedAt) {
		t.Errorf("backfilled published_at = %s, want conservative updated_at %s", got, updatedAt)
	}

	firstStart := window.StartsAt.Add(4 * time.Hour)
	firstStop := firstStart.Add(4 * time.Hour)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at, stopped_at)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, firstStart, firstStop)
	err := db.Exec(`
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at, stopped_at)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, firstStart.Add(time.Hour), firstStop.Add(-time.Hour)).Error
	if err == nil || !strings.Contains(err.Error(), "leaderboard_season_exam_sets_no_overlap") {
		t.Fatalf("overlapping interval error = %v, want restored exclusion constraint", err)
	}
}

func TestPostgresPublishVsProjectionSelfHealsCommittedStatusGap(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	setRepo := examsetrepo.NewAdminRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Now())
	seasonID := uuid.New()

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'draft', false, ?, ?)
	`, examSetID, trackID, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?)`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id, exam_set_id) VALUES (?, ?)`, attemptID, examSetID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)

	if err := setRepo.UpdateStatus(ctx, examSetID, examsetdomain.StatusPublished, true); err != nil {
		t.Fatalf("commit published status: %v", err)
	}
	var publishedAt time.Time
	if err := db.Raw(`SELECT published_at FROM exam_sets WHERE id = ?`, examSetID).Scan(&publishedAt).Error; err != nil {
		t.Fatalf("read published_at: %v", err)
	}
	if publishedAt.IsZero() {
		t.Fatal("published_at was not persisted atomically with published status")
	}
	submittedAt := publishedAt
	if submittedAt.Before(window.StartsAt) || !submittedAt.Before(window.EndsAt) {
		t.Fatalf("test publication %s is outside the current Bangkok season", publishedAt)
	}

	update, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
		AttemptID: attemptID, UserID: userID, ExamSetID: examSetID, ExamTrackID: trackID,
		TrackCode: "integration-track", SubmittedAt: submittedAt,
		Candidate: domain.ScoreCandidate{Points: 91.2, DurationSeconds: 600, AchievedAt: submittedAt},
	})
	if err != nil {
		t.Fatalf("ProjectAttempt() during status-to-hook gap error = %v", err)
	}
	if update.SeasonID != seasonID.String() || update.TotalPoints != 91.2 {
		t.Fatalf("gap projection season/points = %q/%.1f, want %s/91.2", update.SeasonID, update.TotalPoints, seasonID)
	}
	retryHookAt := window.EndsAt.Add(time.Hour)
	if err := projector.OnExamSetPublished(ctx, trackID, examSetID, retryHookAt); err != nil {
		t.Fatalf("later publish hook retry: %v", err)
	}
	var intervalState struct {
		Count    int64
		JoinedAt time.Time
	}
	if err := db.Raw(`
		SELECT COUNT(*) AS count, MIN(joined_at) AS joined_at
		FROM leaderboard_season_exam_sets
		WHERE exam_set_id = ?
	`, examSetID).Scan(&intervalState).Error; err != nil {
		t.Fatalf("read interval state: %v", err)
	}
	if intervalState.Count != 1 || !intervalState.JoinedAt.Equal(publishedAt) {
		t.Errorf(
			"interval state = %d at %s, want one interval at persisted publication %s",
			intervalState.Count,
			intervalState.JoinedAt,
			publishedAt,
		)
	}
}

func TestPostgresStopVsProjectionRetriesEligiblePreStopAttempt(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	setRepo := examsetrepo.NewAdminRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	publishedAt := window.StartsAt
	submittedAt := window.StartsAt.Add(11 * time.Hour)
	stoppedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, published_at, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?, ?)
	`, examSetID, trackID, publishedAt, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?)`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id, exam_set_id) VALUES (?, ?)`, attemptID, examSetID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)

	if err := setRepo.UpdateStatus(ctx, examSetID, examsetdomain.StatusDraft, true); err != nil {
		t.Fatalf("commit unpublished status: %v", err)
	}
	input := domain.ProjectionInput{
		AttemptID: attemptID, UserID: userID, ExamSetID: examSetID, ExamTrackID: trackID,
		TrackCode: "integration-track", SubmittedAt: submittedAt,
		Candidate: domain.ScoreCandidate{Points: 91.2, DurationSeconds: 600, AchievedAt: submittedAt},
	}
	if _, err := projector.ProjectAttempt(ctx, input); !errors.Is(err, leaderboardrepo.ErrLifecycleStatePending) {
		t.Fatalf("projection during stop gap error = %v, want retryable lifecycle error", err)
	}
	if err := projector.OnExamSetStopped(ctx, examSetID, stoppedAt); err != nil {
		t.Fatalf("delayed stop hook: %v", err)
	}
	update, err := projector.ProjectAttempt(ctx, input)
	if err != nil {
		t.Fatalf("eligible pre-stop retry error = %v", err)
	}
	if update.SeasonID != seasonID.String() || update.TotalPoints != 91.2 {
		t.Fatalf("pre-stop retry season/points = %q/%.1f, want %s/91.2", update.SeasonID, update.TotalPoints, seasonID)
	}
}

func TestPostgresPublishRollsBackSeasonWhenIntervalWriteFails(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	projector := leaderboardusecase.NewProjector(leaderboardrepo.NewPostgresRepository(db))
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	publishedAt := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, published_at)
		VALUES (?, ?, 'published', true, ?)
	`, examSetID, trackID, publishedAt)
	mustExec(t, db, `
		CREATE FUNCTION reject_leaderboard_interval() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced interval failure';
		END;
		$$ LANGUAGE plpgsql
	`)
	mustExec(t, db, `
		CREATE TRIGGER reject_leaderboard_interval
		BEFORE INSERT ON leaderboard_season_exam_sets
		FOR EACH ROW EXECUTE FUNCTION reject_leaderboard_interval()
	`)

	err := projector.OnExamSetPublished(ctx, trackID, examSetID, publishedAt)
	if err == nil {
		t.Fatal("OnExamSetPublished() error = nil, want forced interval failure")
	}
	if !strings.Contains(err.Error(), "forced interval failure") {
		t.Fatalf("OnExamSetPublished() error = %v, want injected interval failure", err)
	}
	var seasonCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM leaderboard_seasons WHERE exam_track_id = ?`, trackID).Scan(&seasonCount).Error; err != nil {
		t.Fatalf("count seasons: %v", err)
	}
	if seasonCount != 0 {
		t.Errorf("season rows = %d, want 0 after atomic publish rollback", seasonCount)
	}
}

func TestPostgresPublishRejectsStaleEventsAndPreservesExactRepublish(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	projector := leaderboardusecase.NewProjector(leaderboardrepo.NewPostgresRepository(db))
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	firstPublishedAt := window.StartsAt.Add(8 * time.Hour)
	republishedAt := window.StartsAt.Add(14 * time.Hour)
	stoppedAt := window.StartsAt.Add(20 * time.Hour)
	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, published_at)
		VALUES (?, ?, 'published', true, ?)
	`, examSetID, trackID, republishedAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, firstPublishedAt)

	if err := projector.OnExamSetPublished(ctx, trackID, examSetID, republishedAt); err != nil {
		t.Fatalf("republish: %v", err)
	}
	if err := projector.OnExamSetStopped(ctx, examSetID, stoppedAt); err != nil {
		t.Fatalf("stop republished interval: %v", err)
	}
	for _, retryAt := range []time.Time{firstPublishedAt.Add(time.Hour), republishedAt} {
		if err := projector.OnExamSetPublished(ctx, trackID, examSetID, retryAt); err != nil {
			t.Fatalf("stale/exact publish at %s: %v", retryAt, err)
		}
	}

	type intervalRow struct {
		JoinedAt  time.Time
		StoppedAt *time.Time
	}
	var intervals []intervalRow
	if err := db.Raw(`
		SELECT joined_at, stopped_at
		FROM leaderboard_season_exam_sets
		WHERE season_id = ? AND exam_set_id = ?
		ORDER BY joined_at
	`, seasonID, examSetID).Scan(&intervals).Error; err != nil {
		t.Fatalf("list intervals: %v", err)
	}
	if len(intervals) != 2 {
		t.Fatalf("intervals = %d, want 2 after stale/exact retries", len(intervals))
	}
	if intervals[0].StoppedAt == nil || !intervals[0].StoppedAt.Equal(republishedAt) {
		t.Errorf("first stopped_at = %v, want exact republish %s", intervals[0].StoppedAt, republishedAt)
	}
	if intervals[1].StoppedAt == nil || !intervals[1].StoppedAt.Equal(stoppedAt) {
		t.Errorf("second stopped_at = %v, want %s", intervals[1].StoppedAt, stoppedAt)
	}
	if err := db.Exec(`
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at, stopped_at)
		VALUES (?, ?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, republishedAt.Add(-time.Hour), republishedAt.Add(time.Hour)).Error; err == nil {
		t.Fatal("overlapping interval insert succeeded, want database rejection")
	}
}

func TestPostgresExamSetPublicationTimestampTracksEveryActivationPath(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := examsetrepo.NewAdminRepository(db)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, title, status, is_active)
		VALUES (?, ?, 'publication-state', 'Publication state', 'draft', false)
	`, examSetID, trackID)

	if err := repo.UpdateStatus(ctx, examSetID, examsetdomain.StatusPublished, true); err != nil {
		t.Fatalf("publish through UpdateStatus: %v", err)
	}
	first := readExamSetPublishedAt(t, db, examSetID)
	if first.IsZero() {
		t.Fatal("first published_at is zero")
	}
	if err := repo.UpdateStatus(ctx, examSetID, examsetdomain.StatusPublished, true); err != nil {
		t.Fatalf("retry published status: %v", err)
	}
	if got := readExamSetPublishedAt(t, db, examSetID); !got.Equal(first) {
		t.Errorf("publish retry changed published_at from %s to %s", first, got)
	}
	if err := repo.UpdateStatus(ctx, examSetID, examsetdomain.StatusDraft, true); err != nil {
		t.Fatalf("unpublish before republish: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.UpdateStatus(ctx, examSetID, examsetdomain.StatusPublished, true); err != nil {
		t.Fatalf("republish from draft: %v", err)
	}
	explicitRepublish := readExamSetPublishedAt(t, db, examSetID)
	if !explicitRepublish.After(first) {
		t.Errorf("draft-to-published republish timestamp = %s, want after %s", explicitRepublish, first)
	}

	set := &examsetdomain.ExamSet{
		ID: examSetID, ExamTrackID: trackID, Code: "publication-state", Title: "Edited",
		DurationMinutes: 60, TotalQuestions: 10, PassingScore: 70,
		Difficulty: examsetdomain.DifficultyMedium, AccessType: examsetdomain.AccessFree,
		Currency: "THB", Mode: examsetdomain.ModePractice, IsActive: true,
		AnswerSheetLayout: examsetdomain.DefaultAnswerSheetLayout(),
	}
	if err := repo.Update(ctx, set); err != nil {
		t.Fatalf("unrelated Update: %v", err)
	}
	if got := readExamSetPublishedAt(t, db, examSetID); !got.Equal(explicitRepublish) {
		t.Errorf("unrelated update changed published_at from %s to %s", explicitRepublish, got)
	}

	if err := repo.UpdateIsActive(ctx, examSetID, false); err != nil {
		t.Fatalf("deactivate through UpdateIsActive: %v", err)
	}
	if got := readExamSetPublishedAt(t, db, examSetID); !got.Equal(explicitRepublish) {
		t.Errorf("deactivation changed published_at from %s to %s", explicitRepublish, got)
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.UpdateIsActive(ctx, examSetID, true); err != nil {
		t.Fatalf("reactivate through UpdateIsActive: %v", err)
	}
	second := readExamSetPublishedAt(t, db, examSetID)
	if !second.After(explicitRepublish) {
		t.Errorf("UpdateIsActive reactivation published_at = %s, want after %s", second, explicitRepublish)
	}

	set.IsActive = false
	if err := repo.Update(ctx, set); err != nil {
		t.Fatalf("deactivate through Update: %v", err)
	}
	if got := readExamSetPublishedAt(t, db, examSetID); !got.Equal(second) {
		t.Errorf("general deactivation changed published_at from %s to %s", second, got)
	}
	time.Sleep(2 * time.Millisecond)
	set.IsActive = true
	if err := repo.Update(ctx, set); err != nil {
		t.Fatalf("reactivate through Update: %v", err)
	}
	third := readExamSetPublishedAt(t, db, examSetID)
	if !third.After(second) {
		t.Errorf("Update reactivation published_at = %s, want after %s", third, second)
	}

	mustExec(t, db, `INSERT INTO exam_attempts (id, exam_set_id) VALUES (?, ?)`, uuid.New(), examSetID)
	deactivated, err := repo.Delete(ctx, examSetID)
	if err != nil || !deactivated {
		t.Fatalf("soft Delete() = %t, %v, want true, nil", deactivated, err)
	}
	if got := readExamSetPublishedAt(t, db, examSetID); !got.Equal(third) {
		t.Errorf("soft delete changed published_at from %s to %s", third, got)
	}
}

func TestPostgresProjectionReturnsRankMovementAndRebuiltAggregate(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	competitorID := uuid.New()
	oldAttemptID := uuid.New()
	newAttemptID := uuid.New()
	competitorAttemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	achievedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?), (?)`, userID, competitorID)
	mustExec(t, db, `
		INSERT INTO exam_attempts (id) VALUES (?), (?), (?)
	`, oldAttemptID, newAttemptID, competitorAttemptID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_scores (
			season_id, user_id, exam_set_id, attempt_id, points, duration_seconds, achieved_at
		) VALUES
			(?, ?, ?, ?, 80, 700, ?),
			(?, ?, ?, ?, 85, 650, ?)
	`, seasonID, userID, examSetID, oldAttemptID, achievedAt,
		seasonID, competitorID, examSetID, competitorAttemptID, achievedAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_entries (
			season_id, user_id, total_points, completed_exam_sets,
			total_duration_seconds, score_achieved_at
		) VALUES
			(?, ?, 80, 1, 700, ?),
			(?, ?, 85, 1, 650, ?)
	`, seasonID, userID, achievedAt, seasonID, competitorID, achievedAt)

	update, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
		AttemptID:   newAttemptID,
		UserID:      userID,
		ExamSetID:   examSetID,
		ExamTrackID: trackID,
		TrackCode:   "integration-track",
		SubmittedAt: achievedAt.Add(time.Hour),
		Candidate: domain.ScoreCandidate{
			Points:          90,
			DurationSeconds: 600,
			AchievedAt:      achievedAt.Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("ProjectAttempt() error = %v", err)
	}
	if update.PreviousRank == nil || *update.PreviousRank != 2 {
		t.Errorf("PreviousRank = %v, want 2", update.PreviousRank)
	}
	if update.CurrentRank != 1 {
		t.Errorf("CurrentRank = %d, want 1", update.CurrentRank)
	}
	if update.BestScoreBefore != 80 || update.BestScoreAfter != 90 || update.PointsAdded != 10 {
		t.Errorf(
			"best before/after/added = %.1f/%.1f/%.1f, want 80/90/10",
			update.BestScoreBefore,
			update.BestScoreAfter,
			update.PointsAdded,
		)
	}
	if update.TotalPoints != 90 || !update.ImprovedBestScore {
		t.Errorf("total/improved = %.1f/%t, want 90/true", update.TotalPoints, update.ImprovedBestScore)
	}
}

func TestPostgresProjectionRollsBackScoreWhenEntryRebuildFails(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	projector := leaderboardusecase.NewProjector(repo)
	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()

	trackID := uuid.New()
	examSetID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	seasonID := uuid.New()
	submittedAt := window.StartsAt.Add(12 * time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id) VALUES (?)`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, status, is_active, created_at, updated_at)
		VALUES (?, ?, 'published', true, ?, ?)
	`, examSetID, trackID, window.StartsAt, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id) VALUES (?)`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id) VALUES (?)`, attemptID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (
			id, exam_track_id, year, month, starts_at, ends_at, status
		) VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)
	mustExec(t, db, `
		CREATE FUNCTION reject_leaderboard_entry() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced entry rebuild failure';
		END;
		$$ LANGUAGE plpgsql
	`)
	mustExec(t, db, `
		CREATE TRIGGER reject_leaderboard_entry
		BEFORE INSERT OR UPDATE ON leaderboard_entries
		FOR EACH ROW EXECUTE FUNCTION reject_leaderboard_entry()
	`)

	_, err := projector.ProjectAttempt(ctx, domain.ProjectionInput{
		AttemptID:   attemptID,
		UserID:      userID,
		ExamSetID:   examSetID,
		ExamTrackID: trackID,
		TrackCode:   "integration-track",
		SubmittedAt: submittedAt,
		Candidate: domain.ScoreCandidate{
			Points:          90,
			DurationSeconds: 600,
			AchievedAt:      submittedAt,
		},
	})
	if err == nil {
		t.Fatal("ProjectAttempt() error = nil, want forced aggregate rebuild failure")
	}
	if !strings.Contains(err.Error(), "forced entry rebuild failure") {
		t.Fatalf("ProjectAttempt() error = %v, want injected entry rebuild failure", err)
	}

	for _, table := range []string{"leaderboard_scores", "leaderboard_entries"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM " + table).Scan(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s rows = %d, want 0 after transaction rollback", table, count)
		}
	}
}

func openLeaderboardIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("LEADERBOARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEADERBOARD_POSTGRES_DSN is not set")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	schemaName := "leaderboard_task3_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create integration schema: %v", err)
	}

	testDB, err := gorm.Open(
		postgres.Open(dsnWithSearchPath(dsn, schemaName)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open schema-scoped PostgreSQL connection: %v", err)
	}
	testSQLDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("access schema-scoped sql.DB: %v", err)
	}
	testSQLDB.SetMaxOpenConns(8)

	adminSQLDB, err := admin.DB()
	if err != nil {
		t.Fatalf("access admin sql.DB: %v", err)
	}
	t.Cleanup(func() {
		_ = testSQLDB.Close()
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + schemaName + " CASCADE").Error
		_ = adminSQLDB.Close()
	})

	for _, statement := range []string{
		`CREATE TABLE exam_tracks (id uuid PRIMARY KEY)`,
		`CREATE TABLE exam_sets (
				id uuid PRIMARY KEY,
				exam_track_id uuid NOT NULL REFERENCES exam_tracks(id),
				code varchar(100) NOT NULL DEFAULT '',
				title varchar(255) NOT NULL DEFAULT '',
				description text NOT NULL DEFAULT '',
				cover_image_url text,
				duration_minutes int NOT NULL DEFAULT 1,
				total_questions int NOT NULL DEFAULT 1,
				passing_score int NOT NULL DEFAULT 0,
				difficulty varchar(50) NOT NULL DEFAULT 'easy',
				access_type varchar(50) NOT NULL DEFAULT 'free',
				allow_single_purchase boolean NOT NULL DEFAULT false,
				price_amount numeric NOT NULL DEFAULT 0,
				original_price_amount numeric,
				currency varchar(10) NOT NULL DEFAULT 'THB',
				sale_price_amount numeric,
				mode varchar(50) NOT NULL DEFAULT 'practice',
				is_official boolean NOT NULL DEFAULT false,
				is_featured boolean NOT NULL DEFAULT false,
				status varchar(50) NOT NULL,
				is_active boolean NOT NULL,
				answer_sheet_block_columns int NOT NULL DEFAULT 2,
				answer_sheet_questions_per_block int NOT NULL DEFAULT 10,
				answer_sheet_choice_label_style varchar(20) NOT NULL DEFAULT 'thai',
				answer_sheet_show_header boolean NOT NULL DEFAULT true,
				answer_sheet_show_instructions boolean NOT NULL DEFAULT true,
				answer_sheet_show_candidate_info boolean NOT NULL DEFAULT true,
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now()
			)`,
		`CREATE TABLE users (id uuid PRIMARY KEY)`,
		`CREATE TABLE exam_attempts (id uuid PRIMARY KEY, exam_set_id uuid REFERENCES exam_sets(id))`,
	} {
		mustExec(t, testDB, statement)
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration test file")
	}
	migrationPath := filepath.Join(filepath.Dir(currentFile), "../../../migrations/000023_monthly_leaderboards.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration 000023: %v", err)
	}
	if _, err := testSQLDB.ExecContext(t.Context(), string(migration)); err != nil {
		t.Fatalf("apply migration 000023: %v", err)
	}

	return testDB
}

func dsnWithSearchPath(dsn, schemaName string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schemaName)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return fmt.Sprintf("%s search_path=%s", dsn, schemaName)
}

func mustBangkokWindow(t *testing.T, instant time.Time) domain.SeasonWindow {
	t.Helper()
	window, err := domain.BangkokSeasonWindow(instant)
	if err != nil {
		t.Fatalf("BangkokSeasonWindow() error = %v", err)
	}
	return window
}

func mustExec(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("execute integration fixture SQL: %v", err)
	}
}

func readExamSetPublishedAt(t *testing.T, db *gorm.DB, examSetID uuid.UUID) time.Time {
	t.Helper()
	var publishedAt time.Time
	if err := db.Raw(`SELECT published_at FROM exam_sets WHERE id = ?`, examSetID).Scan(&publishedAt).Error; err != nil {
		t.Fatalf("read exam set published_at: %v", err)
	}
	return publishedAt
}
