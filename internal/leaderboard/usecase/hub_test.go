package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
)

func TestGetOverviewDefaultsToMostRecentlyAttemptedTrack(t *testing.T) {
	fixture := newHubRepositoryFixture()
	fixture.recentTrack = &leaderboardrepo.ExamTrackContextRow{
		ID: fixture.trackID, Code: "civil-service", Name: "Civil Service",
	}
	fixture.rows = []leaderboardrepo.SeasonLeaderboardRow{fixture.entryRow(fixture.userID, 1, 180)}
	fixture.userSummary = fixture.summaryRow(fixture.userID, 1, 180)
	fixture.opportunities = []leaderboardrepo.NextOpportunityRow{{Code: "set-2", Title: "Set 2", ExamTrackName: "Civil Service"}}

	uc := NewLeaderboardUseCase(fixture)
	uc.now = func() time.Time { return fixture.now }

	got, err := uc.GetOverview(t.Context(), fixture.userID)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if got.DefaultTrackCode == nil || *got.DefaultTrackCode != "civil-service" {
		t.Fatalf("DefaultTrackCode = %v, want civil-service", got.DefaultTrackCode)
	}
	if got.ExamTrack == nil || got.ExamTrack.Code != "civil-service" {
		t.Fatalf("ExamTrack = %+v, want civil-service", got.ExamTrack)
	}
	if got.CurrentUser == nil || got.CurrentUser.Rank == nil || *got.CurrentUser.Rank != 1 {
		t.Fatalf("CurrentUser = %+v, want rank 1", got.CurrentUser)
	}
	if len(got.TopThree) != 1 || got.TopThree[0].DisplayName != "Current User" {
		t.Fatalf("TopThree = %+v, want current user", got.TopThree)
	}
	if len(got.NextOpportunities) != 1 || got.NextOpportunities[0].Code != "set-2" {
		t.Fatalf("NextOpportunities = %+v, want set-2", got.NextOpportunities)
	}
	if fixture.ensureSeasonCalls != 1 {
		t.Fatalf("EnsureSeason calls = %d, want 1", fixture.ensureSeasonCalls)
	}
}

func TestGetOverviewWithoutRecentTrackReturnsStableEmptyCollections(t *testing.T) {
	fixture := newHubRepositoryFixture()
	uc := NewLeaderboardUseCase(fixture)
	uc.now = func() time.Time { return fixture.now }

	got, err := uc.GetOverview(t.Context(), fixture.userID)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if got.DefaultTrackCode != nil || got.Season != nil || got.ExamTrack != nil || got.CurrentUser != nil {
		t.Fatalf("GetOverview() = %+v, want no selected track", got)
	}
	if got.TopThree == nil || got.NextOpportunities == nil {
		t.Fatalf("GetOverview() collections must encode as arrays: %+v", got)
	}
	if fixture.listDueCalls != 1 {
		t.Fatalf("ListDueSeasons calls = %d, want 1", fixture.listDueCalls)
	}
}

func TestGetHubUsesExplicitFinalizedSeasonAndCapsTopLimitAtOneHundred(t *testing.T) {
	fixture := newHubRepositoryFixture()
	fixture.season.Status = "finalized"
	finalizedAt := fixture.season.EndsAt.Add(time.Minute)
	fixture.season.FinalizedAt = &finalizedAt
	fixture.rows = []leaderboardrepo.SeasonLeaderboardRow{fixture.entryRow(uuid.New(), 1, 200)}

	uc := NewLeaderboardUseCase(fixture)
	uc.now = func() time.Time { return fixture.now }

	got, err := uc.GetHub(
		t.Context(), fixture.userID, "civil-service", 2026, 6, ScopeTop,
		domain.ListFilter{Page: 2, Limit: 500},
	)
	if err != nil {
		t.Fatalf("GetHub() error = %v", err)
	}
	if fixture.findSeasonYear != 2026 || fixture.findSeasonMonth != 6 {
		t.Fatalf("FindSeason period = %d-%d, want 2026-6", fixture.findSeasonYear, fixture.findSeasonMonth)
	}
	if fixture.listOffset != 100 || fixture.listLimit != 100 {
		t.Fatalf("ListSeasonLeaderboard offset/limit = %d/%d, want 100/100", fixture.listOffset, fixture.listLimit)
	}
	if got.Season.Status != "finalized" || got.Season.FinalizedAt == nil {
		t.Fatalf("Season = %+v, want finalized metadata", got.Season)
	}
	if got.NextOpportunities == nil || len(got.NextOpportunities) != 0 {
		t.Fatalf("NextOpportunities = %+v, want empty finalized list", got.NextOpportunities)
	}
	if fixture.ensureSeasonCalls != 0 {
		t.Fatalf("EnsureSeason calls = %d, want 0 for historical season", fixture.ensureSeasonCalls)
	}
}

func TestGetHubAroundMeReturnsFiveRowsAboveAndBelow(t *testing.T) {
	fixture := newHubRepositoryFixture()
	fixture.userSummary = fixture.summaryRow(fixture.userID, 10, 90)
	for index := 0; index < 11; index++ {
		fixture.aroundRows = append(fixture.aroundRows, fixture.entryRow(uuid.New(), index+5, float64(100-index)))
	}

	uc := NewLeaderboardUseCase(fixture)
	uc.now = func() time.Time { return fixture.now }

	got, err := uc.GetHub(
		t.Context(), fixture.userID, "civil-service", 0, 0, ScopeAroundMe,
		domain.ListFilter{},
	)
	if err != nil {
		t.Fatalf("GetHub() error = %v", err)
	}
	if fixture.aroundAbove != 5 || fixture.aroundBelow != 5 {
		t.Fatalf("around window = %d/%d, want 5/5", fixture.aroundAbove, fixture.aroundBelow)
	}
	if len(got.Leaderboard) != 11 {
		t.Fatalf("Leaderboard rows = %d, want 11", len(got.Leaderboard))
	}
	if got.Pagination.Page != 1 || got.Pagination.Limit != 11 {
		t.Fatalf("Pagination = %+v, want page 1 limit 11", got.Pagination)
	}
}

func TestGetHubAroundMeLeavesUnrankedUserSummaryAndRowsEmpty(t *testing.T) {
	fixture := newHubRepositoryFixture()
	fixture.userSummary = nil
	fixture.aroundRows = nil
	fixture.opportunities = []leaderboardrepo.NextOpportunityRow{{Code: "first-set", Title: "First Set", ExamTrackName: "Civil Service"}}

	uc := NewLeaderboardUseCase(fixture)
	uc.now = func() time.Time { return fixture.now }

	got, err := uc.GetHub(
		t.Context(), fixture.userID, "civil-service", 0, 0, ScopeAroundMe,
		domain.ListFilter{},
	)
	if err != nil {
		t.Fatalf("GetHub() error = %v", err)
	}
	if got.CurrentUser != nil {
		t.Fatalf("CurrentUser = %+v, want nil", got.CurrentUser)
	}
	if got.Leaderboard == nil || len(got.Leaderboard) != 0 {
		t.Fatalf("Leaderboard = %+v, want stable empty array", got.Leaderboard)
	}
	if len(got.NextOpportunities) != 1 || got.NextOpportunities[0].Code != "first-set" {
		t.Fatalf("NextOpportunities = %+v, want first-set", got.NextOpportunities)
	}
}

func TestListAwardsMapsSharedRankMedalsInDeterministicOrder(t *testing.T) {
	fixture := newHubRepositoryFixture()
	fixture.awards = []leaderboardrepo.AwardRow{
		{SeasonID: uuid.New(), TrackCode: "civil-service", Year: 2026, Month: 6, Rank: 1, AwardedAt: fixture.now},
		{SeasonID: uuid.New(), TrackCode: "teacher", Year: 2026, Month: 5, Rank: 2, AwardedAt: fixture.now.Add(-time.Hour)},
		{SeasonID: uuid.New(), TrackCode: "teacher", Year: 2026, Month: 5, Rank: 2, AwardedAt: fixture.now.Add(-time.Hour)},
	}

	got, err := NewLeaderboardUseCase(fixture).ListAwards(t.Context(), fixture.userID)
	if err != nil {
		t.Fatalf("ListAwards() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListAwards() rows = %d, want 3", len(got))
	}
	for index, want := range []string{"gold", "silver", "silver"} {
		if got[index].Medal != want {
			t.Errorf("awards[%d].Medal = %q, want %q", index, got[index].Medal, want)
		}
	}
}

type hubRepositoryFixture struct {
	leaderboardrepo.Repository

	now      time.Time
	trackID  uuid.UUID
	seasonID uuid.UUID
	userID   uuid.UUID

	recentTrack   *leaderboardrepo.ExamTrackContextRow
	season        leaderboardrepo.SeasonRow
	rows          []leaderboardrepo.SeasonLeaderboardRow
	aroundRows    []leaderboardrepo.SeasonLeaderboardRow
	userSummary   *leaderboardrepo.SeasonUserSummaryRow
	opportunities []leaderboardrepo.NextOpportunityRow
	awards        []leaderboardrepo.AwardRow

	ensureSeasonCalls int
	findSeasonYear    int
	findSeasonMonth   int
	listOffset        int
	listLimit         int
	aroundAbove       int
	aroundBelow       int
	listDueCalls      int
}

func newHubRepositoryFixture() *hubRepositoryFixture {
	now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	window, err := domain.BangkokSeasonWindow(now)
	if err != nil {
		panic(err)
	}
	fixture := &hubRepositoryFixture{
		now:      now,
		trackID:  uuid.New(),
		seasonID: uuid.New(),
		userID:   uuid.New(),
	}
	fixture.season = leaderboardrepo.SeasonRow{
		ID: fixture.seasonID, ExamTrackID: fixture.trackID,
		Year: window.Year, Month: window.Month, StartsAt: window.StartsAt, EndsAt: window.EndsAt,
		Status: "active",
	}
	return fixture
}

func (f *hubRepositoryFixture) entryRow(userID uuid.UUID, rank int, points float64) leaderboardrepo.SeasonLeaderboardRow {
	return leaderboardrepo.SeasonLeaderboardRow{
		Rank: rank, UserID: userID, DisplayName: map[bool]string{true: "Current User", false: "Other User"}[userID == f.userID],
		Email: "member@example.com", TotalPoints: points, CompletedExamSets: 2,
		TotalDurationSeconds: 900, ScoreAchievedAt: f.now.Add(-time.Hour),
	}
}

func (f *hubRepositoryFixture) summaryRow(userID uuid.UUID, rank int, points float64) *leaderboardrepo.SeasonUserSummaryRow {
	return &leaderboardrepo.SeasonUserSummaryRow{
		Rank: rank, UserID: userID, TotalPoints: points, CompletedExamSets: 2,
		TotalDurationSeconds: 900, ScoreAchievedAt: f.now.Add(-time.Hour),
	}
}

func (f *hubRepositoryFixture) FindMostRecentAttemptedTrack(context.Context, uuid.UUID) (*leaderboardrepo.ExamTrackContextRow, error) {
	return f.recentTrack, nil
}

func (f *hubRepositoryFixture) EnsureSeason(_ context.Context, _ uuid.UUID, _ domain.SeasonWindow) (*leaderboardrepo.SeasonRow, error) {
	f.ensureSeasonCalls++
	row := f.season
	return &row, nil
}

func (f *hubRepositoryFixture) FindSeason(_ context.Context, _ uuid.UUID, year, month int) (*leaderboardrepo.SeasonRow, error) {
	f.findSeasonYear = year
	f.findSeasonMonth = month
	row := f.season
	row.Year = year
	row.Month = month
	return &row, nil
}

func (f *hubRepositoryFixture) FindActiveExamTrackByCode(context.Context, string) (*leaderboardrepo.ExamTrackContextRow, error) {
	return &leaderboardrepo.ExamTrackContextRow{ID: f.trackID, Code: "civil-service", Name: "Civil Service"}, nil
}

func (f *hubRepositoryFixture) CountSeasonLeaderboard(context.Context, uuid.UUID) (int64, error) {
	return int64(max(len(f.rows), len(f.aroundRows))), nil
}

func (f *hubRepositoryFixture) ListSeasonLeaderboard(_ context.Context, _ uuid.UUID, offset, limit int) ([]leaderboardrepo.SeasonLeaderboardRow, error) {
	f.listOffset = offset
	f.listLimit = limit
	return f.rows, nil
}

func (f *hubRepositoryFixture) ListSeasonTopThree(context.Context, uuid.UUID) ([]leaderboardrepo.SeasonLeaderboardRow, error) {
	rows := make([]leaderboardrepo.SeasonLeaderboardRow, 0, len(f.rows))
	for _, row := range f.rows {
		if row.Rank <= 3 {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (f *hubRepositoryFixture) ListSeasonLeaderboardAroundUser(_ context.Context, _, _ uuid.UUID, above, below int) ([]leaderboardrepo.SeasonLeaderboardRow, error) {
	f.aroundAbove = above
	f.aroundBelow = below
	return f.aroundRows, nil
}

func (f *hubRepositoryFixture) GetSeasonUserSummary(context.Context, uuid.UUID, uuid.UUID) (*leaderboardrepo.SeasonUserSummaryRow, error) {
	return f.userSummary, nil
}

func (f *hubRepositoryFixture) ListNextOpportunities(context.Context, uuid.UUID, uuid.UUID) ([]leaderboardrepo.NextOpportunityRow, error) {
	return f.opportunities, nil
}

func (f *hubRepositoryFixture) ListAwards(context.Context, uuid.UUID) ([]leaderboardrepo.AwardRow, error) {
	return f.awards, nil
}

func (f *hubRepositoryFixture) ListDueSeasons(context.Context, time.Time) ([]leaderboardrepo.SeasonRow, error) {
	f.listDueCalls++
	return nil, nil
}

func (f *hubRepositoryFixture) FinalizeSeason(context.Context, uuid.UUID, time.Time) (*leaderboardrepo.FinalizationResult, error) {
	return &leaderboardrepo.FinalizationResult{}, nil
}
