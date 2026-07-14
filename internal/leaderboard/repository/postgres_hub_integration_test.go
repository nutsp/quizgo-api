package repository_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

func TestPostgresHubRankingCalculatesSharedRanksStableOrderAndActiveVisibility(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	trackID := uuid.New()
	seasonID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC))
	tiedAt := window.StartsAt.Add(10 * time.Hour)
	users := []struct {
		id     uuid.UUID
		status string
		points float64
	}{
		{uuid.MustParse("00000000-0000-0000-0000-000000000004"), "active", 80},
		{uuid.MustParse("00000000-0000-0000-0000-000000000003"), "active", 90},
		{uuid.MustParse("00000000-0000-0000-0000-000000000002"), "active", 90},
		{uuid.MustParse("00000000-0000-0000-0000-000000000001"), "disabled", 100},
	}

	mustExec(t, db, `INSERT INTO exam_tracks (id, code, name) VALUES (?, 'civil-service', 'Civil Service')`, trackID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	for _, user := range users {
		mustExec(t, db, `
			INSERT INTO users (id, display_name, email, status) VALUES (?, ?, ?, ?)
		`, user.id, "User "+user.id.String(), user.id.String()+"@example.com", user.status)
		mustExec(t, db, `
			INSERT INTO leaderboard_entries (
				season_id, user_id, total_points, completed_exam_sets,
				total_duration_seconds, score_achieved_at
			) VALUES (?, ?, ?, 1, 600, ?)
		`, seasonID, user.id, user.points, tiedAt)
	}

	rows, err := repo.ListSeasonLeaderboard(t.Context(), seasonID, 0, 100)
	if err != nil {
		t.Fatalf("ListSeasonLeaderboard() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("active rows = %d, want 3", len(rows))
	}
	for index, wantRank := range []int{1, 1, 3} {
		if rows[index].Rank != wantRank {
			t.Errorf("active rows[%d].Rank = %d, want %d", index, rows[index].Rank, wantRank)
		}
	}
	if rows[0].UserID.String() >= rows[1].UserID.String() {
		t.Errorf("shared rank order = %s then %s, want ascending user ID", rows[0].UserID, rows[1].UserID)
	}
	if rows[0].UserID == users[3].id || rows[1].UserID == users[3].id || rows[2].UserID == users[3].id {
		t.Fatal("disabled user was included in active-season public rows")
	}

	count, err := repo.CountSeasonLeaderboard(t.Context(), seasonID)
	if err != nil {
		t.Fatalf("CountSeasonLeaderboard() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("active count = %d, want 3", count)
	}

	finalizedAt := window.EndsAt.Add(time.Minute)
	mustExec(t, db, `UPDATE leaderboard_seasons SET status = 'finalized', finalized_at = ? WHERE id = ?`, finalizedAt, seasonID)
	rows, err = repo.ListSeasonLeaderboard(t.Context(), seasonID, 0, 100)
	if err != nil {
		t.Fatalf("ListSeasonLeaderboard() finalized error = %v", err)
	}
	if len(rows) != 4 || rows[0].UserID != users[3].id || rows[0].Rank != 1 {
		t.Fatalf("finalized rows = %+v, want retained disabled historical leader", rows)
	}
}

func TestPostgresAroundMeReturnsFiveStableRowsOnEachSide(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	trackID := uuid.New()
	seasonID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC))
	userIDs := make([]uuid.UUID, 15)

	mustExec(t, db, `INSERT INTO exam_tracks (id, code, name) VALUES (?, 'civil-service', 'Civil Service')`, trackID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	for index := range userIDs {
		userIDs[index] = uuid.MustParse("20000000-0000-0000-0000-" + fmt.Sprintf("%012d", index+1))
		mustExec(t, db, `INSERT INTO users (id, display_name, email) VALUES (?, ?, ?)`, userIDs[index], "User", userIDs[index].String()+"@example.com")
		mustExec(t, db, `
			INSERT INTO leaderboard_entries (
				season_id, user_id, total_points, completed_exam_sets,
				total_duration_seconds, score_achieved_at
			) VALUES (?, ?, ?, 1, 600, ?)
		`, seasonID, userIDs[index], 100-index, window.StartsAt.Add(time.Hour))
	}

	rows, err := repo.ListSeasonLeaderboardAroundUser(t.Context(), seasonID, userIDs[7], 5, 5)
	if err != nil {
		t.Fatalf("ListSeasonLeaderboardAroundUser() error = %v", err)
	}
	if len(rows) != 11 {
		t.Fatalf("around-me rows = %d, want 11", len(rows))
	}
	for index, row := range rows {
		wantUserID := userIDs[index+2]
		wantRank := index + 3
		if row.UserID != wantUserID || row.Rank != wantRank {
			t.Errorf("rows[%d] = user %s rank %d, want user %s rank %d", index, row.UserID, row.Rank, wantUserID, wantRank)
		}
	}
}

func TestPostgresFinalizationPreventsQueuedProjectionFromMutatingFinalizedSeason(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	trackID := uuid.New()
	examSetID := uuid.New()
	seasonID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.June, 15, 10, 0, 0, 0, time.UTC))
	submittedAt := window.EndsAt.Add(-time.Hour)
	finalizedAt := window.EndsAt.Add(time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id, code, name) VALUES (?, 'civil-service', 'Civil Service')`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, title, status, is_active, published_at)
		VALUES (?, ?, 'set-1', 'Set 1', 'published', true, ?)
	`, examSetID, trackID, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id, display_name, email) VALUES (?, 'User', 'user@example.com')`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id, exam_set_id) VALUES (?, ?)`, attemptID, examSetID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)

	blocker := db.Begin()
	if blocker.Error != nil {
		t.Fatalf("begin projection lock blocker: %v", blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if err := blocker.Exec(`
		SELECT pg_advisory_xact_lock(hashtextextended(CAST(? AS text), 1))
	`, seasonID).Error; err != nil {
		t.Fatalf("acquire projection lock blocker: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()
	finalized := make(chan *leaderboardrepo.FinalizationResult, 1)
	finalizeErr := make(chan error, 1)
	go func() {
		result, err := repo.FinalizeSeason(ctx, seasonID, finalizedAt)
		finalized <- result
		finalizeErr <- err
	}()
	waitForPostgresLockWait(t, db, "pg_advisory_xact_lock", "hashtextextended")

	projected := make(chan *leaderboardrepo.BestScoreProjection, 1)
	projectionErr := make(chan error, 1)
	go func() {
		result, err := repo.ProjectBestScore(
			ctx,
			userID,
			examSetID,
			attemptID,
			submittedAt,
			domain.ScoreCandidate{Points: 95, DurationSeconds: 600, AchievedAt: submittedAt},
		)
		projected <- result
		projectionErr <- err
	}()
	waitForPostgresLockWaitCount(t, db, 2, "pg_advisory_xact_lock", "hashtextextended")
	if err := blocker.Commit().Error; err != nil {
		t.Fatalf("release projection lock blocker: %v", err)
	}

	if err := <-finalizeErr; err != nil {
		t.Fatalf("FinalizeSeason() error = %v", err)
	}
	if result := <-finalized; result == nil || !result.Finalized {
		t.Fatalf("FinalizeSeason() result = %+v, want finalized transition", result)
	}
	if err := <-projectionErr; err != nil {
		t.Fatalf("ProjectBestScore() error = %v", err)
	}
	if result := <-projected; result == nil || result.Season != nil {
		t.Fatalf("ProjectBestScore() result = %+v, want finalized-season no-op", result)
	}

	for _, table := range []string{"leaderboard_scores", "leaderboard_entries"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE season_id = ?", seasonID).Scan(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s rows after finalization = %d, want 0", table, count)
		}
	}
}

func TestPostgresProjectionQueuedBeforeFinalizationIsIncludedWithoutDeadlock(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	trackID := uuid.New()
	examSetID := uuid.New()
	seasonID := uuid.New()
	userID := uuid.New()
	attemptID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.June, 15, 10, 0, 0, 0, time.UTC))
	submittedAt := window.EndsAt.Add(-time.Hour)
	finalizedAt := window.EndsAt.Add(time.Hour)

	mustExec(t, db, `INSERT INTO exam_tracks (id, code, name) VALUES (?, 'civil-service', 'Civil Service')`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, title, status, is_active, published_at)
		VALUES (?, ?, 'set-1', 'Set 1', 'published', true, ?)
	`, examSetID, trackID, window.StartsAt)
	mustExec(t, db, `INSERT INTO users (id, display_name, email) VALUES (?, 'User', 'user@example.com')`, userID)
	mustExec(t, db, `INSERT INTO exam_attempts (id, exam_set_id) VALUES (?, ?)`, attemptID, examSetID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	mustExec(t, db, `
		INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
		VALUES (?, ?, ?, ?)
	`, uuid.New(), seasonID, examSetID, window.StartsAt)

	blocker := db.Begin()
	if blocker.Error != nil {
		t.Fatalf("begin projection lock blocker: %v", blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if err := blocker.Exec(`
		SELECT pg_advisory_xact_lock(hashtextextended(CAST(? AS text), 1))
	`, seasonID).Error; err != nil {
		t.Fatalf("acquire projection lock blocker: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), postgresRaceBound)
	defer cancel()
	projected := make(chan *leaderboardrepo.BestScoreProjection, 1)
	projectionErr := make(chan error, 1)
	go func() {
		result, err := repo.ProjectBestScore(
			ctx,
			userID,
			examSetID,
			attemptID,
			submittedAt,
			domain.ScoreCandidate{Points: 95, DurationSeconds: 600, AchievedAt: submittedAt},
		)
		projected <- result
		projectionErr <- err
	}()
	waitForPostgresLockWait(t, db, "pg_advisory_xact_lock", "hashtextextended")

	finalized := make(chan *leaderboardrepo.FinalizationResult, 1)
	finalizeErr := make(chan error, 1)
	go func() {
		result, err := repo.FinalizeSeason(ctx, seasonID, finalizedAt)
		finalized <- result
		finalizeErr <- err
	}()
	waitForPostgresLockWaitCount(t, db, 2, "pg_advisory_xact_lock", "hashtextextended")
	if err := blocker.Commit().Error; err != nil {
		t.Fatalf("release projection lock blocker: %v", err)
	}

	if err := <-projectionErr; err != nil {
		t.Fatalf("ProjectBestScore() error = %v", err)
	}
	if result := <-projected; result == nil || result.Season == nil {
		t.Fatalf("ProjectBestScore() result = %+v, want included projection", result)
	}
	if err := <-finalizeErr; err != nil {
		t.Fatalf("FinalizeSeason() error = %v", err)
	}
	if result := <-finalized; result == nil || !result.Finalized {
		t.Fatalf("FinalizeSeason() result = %+v, want finalized transition", result)
	}

	var awardCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM leaderboard_awards WHERE season_id = ? AND user_id = ?`, seasonID, userID).Scan(&awardCount).Error; err != nil {
		t.Fatalf("count projected user award: %v", err)
	}
	if awardCount != 1 {
		t.Fatalf("projected user awards = %d, want 1", awardCount)
	}
}

func TestPostgresFinalizeSeasonRebuildsAndAwardsSharedRanksConcurrentlyAndIdempotently(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	trackID := uuid.New()
	examSetID := uuid.New()
	seasonID := uuid.New()
	window := mustBangkokWindow(t, time.Date(2026, time.June, 15, 10, 0, 0, 0, time.UTC))
	finalizedAt := window.EndsAt.Add(time.Hour)
	tiedAt := window.StartsAt.Add(10 * time.Hour)
	users := []struct {
		id     uuid.UUID
		points float64
	}{
		{uuid.MustParse("10000000-0000-0000-0000-000000000001"), 100},
		{uuid.MustParse("10000000-0000-0000-0000-000000000002"), 90},
		{uuid.MustParse("10000000-0000-0000-0000-000000000003"), 90},
		{uuid.MustParse("10000000-0000-0000-0000-000000000004"), 80},
	}

	mustExec(t, db, `INSERT INTO exam_tracks (id, code, name) VALUES (?, 'civil-service', 'Civil Service')`, trackID)
	mustExec(t, db, `
		INSERT INTO exam_sets (id, exam_track_id, code, title, status, is_active)
		VALUES (?, ?, 'set-1', 'Set 1', 'published', true)
	`, examSetID, trackID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	for _, user := range users {
		attemptID := uuid.New()
		mustExec(t, db, `INSERT INTO users (id, display_name, email) VALUES (?, ?, ?)`, user.id, "User", user.id.String()+"@example.com")
		mustExec(t, db, `INSERT INTO exam_attempts (id, exam_set_id) VALUES (?, ?)`, attemptID, examSetID)
		mustExec(t, db, `
			INSERT INTO leaderboard_scores (
				season_id, user_id, exam_set_id, attempt_id, points, duration_seconds, achieved_at
			) VALUES (?, ?, ?, ?, ?, 600, ?)
		`, seasonID, user.id, examSetID, attemptID, user.points, tiedAt)
	}

	results := make(chan *leaderboardrepo.FinalizationResult, 2)
	errs := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			start.Wait()
			result, err := repo.FinalizeSeason(t.Context(), seasonID, finalizedAt)
			results <- result
			errs <- err
		}()
	}
	start.Done()
	workers.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("FinalizeSeason() concurrent error = %v", err)
		}
	}
	finalizedCount := 0
	for result := range results {
		if result != nil && result.Finalized {
			finalizedCount++
		}
	}
	if finalizedCount != 1 {
		t.Fatalf("successful finalization transitions = %d, want 1", finalizedCount)
	}

	var season struct {
		Status      string
		FinalizedAt *time.Time
	}
	if err := db.Raw(`SELECT status, finalized_at FROM leaderboard_seasons WHERE id = ?`, seasonID).Scan(&season).Error; err != nil {
		t.Fatalf("read finalized season: %v", err)
	}
	if season.Status != "finalized" || season.FinalizedAt == nil || !season.FinalizedAt.Equal(finalizedAt) {
		t.Fatalf("finalized season = %+v, want finalized at %s", season, finalizedAt)
	}

	var awards []struct {
		UserID uuid.UUID
		Rank   int
	}
	if err := db.Raw(`
		SELECT user_id, rank FROM leaderboard_awards WHERE season_id = ? ORDER BY rank, user_id
	`, seasonID).Scan(&awards).Error; err != nil {
		t.Fatalf("read awards: %v", err)
	}
	if len(awards) != 3 {
		t.Fatalf("awards = %+v, want three recipients", awards)
	}
	for index, wantRank := range []int{1, 2, 2} {
		if awards[index].Rank != wantRank || awards[index].UserID != users[index].id {
			t.Errorf("awards[%d] = %+v, want user %s rank %d", index, awards[index], users[index].id, wantRank)
		}
	}

	var entryCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM leaderboard_entries WHERE season_id = ?`, seasonID).Scan(&entryCount).Error; err != nil {
		t.Fatalf("count rebuilt entries: %v", err)
	}
	if entryCount != 4 {
		t.Fatalf("rebuilt entries = %d, want 4", entryCount)
	}

	retry, err := repo.FinalizeSeason(t.Context(), seasonID, finalizedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("FinalizeSeason() retry error = %v", err)
	}
	if retry.Finalized {
		t.Fatal("FinalizeSeason() retry reported a second transition")
	}
	var awardCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM leaderboard_awards WHERE season_id = ?`, seasonID).Scan(&awardCount).Error; err != nil {
		t.Fatalf("count retry awards: %v", err)
	}
	if awardCount != 3 {
		t.Fatalf("awards after retry = %d, want 3", awardCount)
	}
}

func TestPostgresHubSeasonAndOpportunityQueriesUseDeterministicOrdering(t *testing.T) {
	db := openLeaderboardIntegrationDB(t)
	repo := leaderboardrepo.NewPostgresRepository(db)
	trackID := uuid.New()
	seasonID := uuid.New()
	userID := uuid.New()
	window, err := domain.BangkokSeasonWindow(time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BangkokSeasonWindow() error = %v", err)
	}

	mustExec(t, db, `INSERT INTO exam_tracks (id, code, name) VALUES (?, 'civil-service', 'Civil Service')`, trackID)
	mustExec(t, db, `INSERT INTO users (id, display_name, email) VALUES (?, 'Current User', 'current@example.com')`, userID)
	mustExec(t, db, `
		INSERT INTO leaderboard_seasons (id, exam_track_id, year, month, starts_at, ends_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, seasonID, trackID, window.Year, window.Month, window.StartsAt, window.EndsAt)
	for index, code := range []string{"set-b", "set-a"} {
		examSetID := uuid.New()
		mustExec(t, db, `
			INSERT INTO exam_sets (id, exam_track_id, code, title, status, is_active)
			VALUES (?, ?, ?, ?, 'published', true)
		`, examSetID, trackID, code, "Set "+string(rune('B'-index)))
		mustExec(t, db, `
			INSERT INTO leaderboard_season_exam_sets (id, season_id, exam_set_id, joined_at)
			VALUES (?, ?, ?, ?)
		`, uuid.New(), seasonID, examSetID, window.StartsAt)
	}

	season, err := repo.FindSeason(t.Context(), trackID, window.Year, window.Month)
	if err != nil || season == nil || season.ID != seasonID {
		t.Fatalf("FindSeason() = %+v, %v, want %s", season, err, seasonID)
	}
	opportunities, err := repo.ListNextOpportunities(t.Context(), seasonID, userID)
	if err != nil {
		t.Fatalf("ListNextOpportunities() error = %v", err)
	}
	if len(opportunities) != 2 || opportunities[0].Code != "set-a" || opportunities[1].Code != "set-b" {
		t.Fatalf("opportunities = %+v, want code order set-a,set-b", opportunities)
	}
}
