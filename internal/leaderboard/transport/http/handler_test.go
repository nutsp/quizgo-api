package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboarduc "virtual-exam-api/internal/leaderboard/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

func TestRegisterRoutesRequiresAuthenticationAndPreservesOnlySupportedLeaderboardRoutes(t *testing.T) {
	service := newHandlerServiceFake()
	e := newLeaderboardTestServer(service, allowAllReadLimiter{})

	paths := []string{
		"/api/v1/leaderboards/overview",
		"/api/v1/leaderboards/exam-tracks/civil-service",
		"/api/v1/me/leaderboard-awards",
		"/api/v1/exam-sets/set-1/leaderboard",
	}
	for _, path := range paths {
		recorder := performLeaderboardRequest(e, path, false)
		if recorder.Code != stdhttp.StatusUnauthorized {
			t.Errorf("GET %s status = %d, want 401", path, recorder.Code)
		}
	}

	registered := make([]string, 0)
	for _, route := range e.Routes() {
		if route.Method == stdhttp.MethodGet {
			registered = append(registered, route.Path)
		}
	}
	sort.Strings(registered)
	if !containsString(registered, "/api/v1/exam-sets/:examSetCode/leaderboard") {
		t.Fatalf("registered routes = %v, missing per-set leaderboard", registered)
	}
	if containsString(registered, "/api/v1/exam-tracks/:trackCode/leaderboard") {
		t.Fatalf("registered routes = %v, legacy track-average route is still public", registered)
	}
}

func TestGetOverviewAndAwardsReturnStableJSONContracts(t *testing.T) {
	service := newHandlerServiceFake()
	e := newLeaderboardTestServer(service, allowAllReadLimiter{})

	overview := performLeaderboardRequest(e, "/api/v1/leaderboards/overview", true)
	if overview.Code != stdhttp.StatusOK {
		t.Fatalf("overview status = %d body=%s", overview.Code, overview.Body.String())
	}
	data := decodeDataObject(t, overview)
	for _, key := range []string{
		"default_track_code", "season", "exam_track", "current_user", "top_three", "next_opportunities",
	} {
		if _, ok := data[key]; !ok {
			t.Errorf("overview JSON missing %q: %v", key, data)
		}
	}

	awards := performLeaderboardRequest(e, "/api/v1/me/leaderboard-awards", true)
	if awards.Code != stdhttp.StatusOK {
		t.Fatalf("awards status = %d body=%s", awards.Code, awards.Body.String())
	}
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(awards.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode awards: %v", err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("awards rows = %d, want 1", len(envelope.Data))
	}
	for _, key := range []string{"season_id", "track_code", "year", "month", "rank", "medal", "awarded_at"} {
		if _, ok := envelope.Data[0][key]; !ok {
			t.Errorf("award JSON missing %q: %v", key, envelope.Data[0])
		}
	}
}

func TestGetHubParsesExplicitQueryAndReturnsStableJSON(t *testing.T) {
	service := newHandlerServiceFake()
	e := newLeaderboardTestServer(service, allowAllReadLimiter{})
	recorder := performLeaderboardRequest(
		e,
		"/api/v1/leaderboards/exam-tracks/civil-service?year=2026&month=6&scope=around_me&page=2&limit=99",
		true,
	)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("GetHub status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.hubYear != 2026 || service.hubMonth != 6 || service.hubScope != leaderboarduc.ScopeAroundMe ||
		service.hubFilter.Page != 2 || service.hubFilter.Limit != 99 {
		t.Fatalf(
			"hub query = %d/%d %q %+v, want 2026/6 around_me page 2 limit 99",
			service.hubYear, service.hubMonth, service.hubScope, service.hubFilter,
		)
	}
	data := decodeDataObject(t, recorder)
	for _, key := range []string{
		"season", "exam_track", "current_user", "top_three", "leaderboard", "next_opportunities", "pagination",
	} {
		if _, ok := data[key]; !ok {
			t.Errorf("hub JSON missing %q: %v", key, data)
		}
	}
	season, ok := data["season"].(map[string]any)
	if !ok {
		t.Fatalf("season JSON = %T, want object", data["season"])
	}
	for _, key := range []string{"year", "month", "starts_at", "ends_at", "status", "finalized_at"} {
		if _, ok := season[key]; !ok {
			t.Errorf("season JSON missing %q: %v", key, season)
		}
	}
}

func TestGetHubRejectsInvalidQueriesWithoutCallingUseCase(t *testing.T) {
	tests := []string{
		"year=abc&month=6",
		"year=2026",
		"year=0&month=6",
		"year=2026&month=13",
		"scope=global",
		"page=0",
		"limit=nope",
		"unexpected=true",
	}
	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			service := newHandlerServiceFake()
			e := newLeaderboardTestServer(service, allowAllReadLimiter{})
			recorder := performLeaderboardRequest(
				e,
				"/api/v1/leaderboards/exam-tracks/civil-service?"+query,
				true,
			)
			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("status = %d body=%s, want 400", recorder.Code, recorder.Body.String())
			}
			if service.hubCalls != 0 {
				t.Fatalf("GetHub calls = %d, want 0", service.hubCalls)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error.Code != "VALIDATION_ERROR" {
				t.Fatalf("error code = %q, want VALIDATION_ERROR", body.Error.Code)
			}
		})
	}
}

func TestPerSetLeaderboardRejectsUnsupportedQuery(t *testing.T) {
	service := newHandlerServiceFake()
	e := newLeaderboardTestServer(service, allowAllReadLimiter{})
	recorder := performLeaderboardRequest(
		e,
		"/api/v1/exam-sets/set-1/leaderboard?scope=top",
		true,
	)
	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", recorder.Code, recorder.Body.String())
	}
}

func TestAuthenticatedLeaderboardReadReturns429OnRequest121AndResetsNextMinute(t *testing.T) {
	service := newHandlerServiceFake()
	store := &rateLimitStoreFake{counts: make(map[string]int64)}
	now := time.Date(2026, time.July, 14, 10, 5, 30, 0, time.UTC)
	limiter := newReadRateLimiter(store, nil, func() time.Time { return now })
	e := newLeaderboardTestServer(service, limiter)

	for request := 1; request <= 120; request++ {
		recorder := performLeaderboardRequest(e, "/api/v1/leaderboards/overview", true)
		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("request %d status = %d body=%s", request, recorder.Code, recorder.Body.String())
		}
	}
	limited := performLeaderboardRequest(e, "/api/v1/leaderboards/overview", true)
	if limited.Code != stdhttp.StatusTooManyRequests {
		t.Fatalf("request 121 status = %d body=%s, want 429", limited.Code, limited.Body.String())
	}

	now = now.Add(time.Minute)
	reset := performLeaderboardRequest(e, "/api/v1/leaderboards/overview", true)
	if reset.Code != stdhttp.StatusOK {
		t.Fatalf("next-minute status = %d body=%s, want 200", reset.Code, reset.Body.String())
	}
}

type handlerServiceFake struct {
	userID uuid.UUID
	now    time.Time

	hubCalls  int
	hubYear   int
	hubMonth  int
	hubScope  string
	hubFilter domain.ListFilter
}

func newHandlerServiceFake() *handlerServiceFake {
	return &handlerServiceFake{
		userID: uuid.New(),
		now:    time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC),
	}
}

func (f *handlerServiceFake) GetOverview(context.Context, uuid.UUID) (*domain.OverviewResponse, error) {
	code := "civil-service"
	season := testSeason(f.now)
	track := domain.ExamTrackRef{Code: code, Name: "Civil Service"}
	return &domain.OverviewResponse{
		DefaultTrackCode:  &code,
		Season:            &season,
		ExamTrack:         &track,
		TopThree:          []domain.LeaderboardEntry{},
		NextOpportunities: []domain.ExamSetRef{},
	}, nil
}

func (f *handlerServiceFake) GetHub(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	year, month int,
	scope string,
	filter domain.ListFilter,
) (*domain.HubResponse, error) {
	f.hubCalls++
	f.hubYear = year
	f.hubMonth = month
	f.hubScope = scope
	f.hubFilter = filter
	return &domain.HubResponse{
		Season:            testSeason(f.now),
		ExamTrack:         domain.ExamTrackRef{Code: "civil-service", Name: "Civil Service"},
		TopThree:          []domain.LeaderboardEntry{},
		Leaderboard:       []domain.LeaderboardEntry{},
		NextOpportunities: []domain.ExamSetRef{},
		Pagination:        domain.Pagination{Page: 1, Limit: 20},
	}, nil
}

func (f *handlerServiceFake) ListAwards(context.Context, uuid.UUID) ([]domain.Award, error) {
	return []domain.Award{{
		SeasonID: uuid.NewString(), TrackCode: "civil-service", Year: 2026, Month: 6,
		Rank: 1, Medal: "gold", AwardedAt: f.now,
	}}, nil
}

func (f *handlerServiceFake) GetExamSetLeaderboard(context.Context, uuid.UUID, string, domain.ListFilter) (*domain.ExamSetLeaderboardResponse, error) {
	return &domain.ExamSetLeaderboardResponse{Leaderboard: []domain.ExamSetLeaderboardEntry{}}, nil
}

func testSeason(now time.Time) domain.SeasonWindow {
	return domain.SeasonWindow{
		Year: 2026, Month: 7, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
		Status: "active", FinalizedAt: nil,
	}
}

type allowAllReadLimiter struct{}

func (allowAllReadLimiter) Allow(context.Context, uuid.UUID) bool { return true }

func newLeaderboardTestServer(service LeaderboardService, limiter ReadLimiter) *echo.Echo {
	e := echo.New()
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get(echo.HeaderAuthorization) != "Bearer valid" {
				return response.Error(c, apperrors.ErrUnauthorized)
			}
			c.Set(middleware.UserIDKey, service.(*handlerServiceFake).userID)
			return next(c)
		}
	}
	NewHandler(service, limiter).RegisterRoutes(e.Group("/api/v1"), auth)
	return e
}

func performLeaderboardRequest(e *echo.Echo, path string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(stdhttp.MethodGet, path, nil)
	if authenticated {
		req.Header.Set(echo.HeaderAuthorization, "Bearer valid")
	}
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, req)
	return recorder
}

func decodeDataObject(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return envelope.Data
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
