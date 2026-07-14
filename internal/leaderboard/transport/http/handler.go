package http

import (
	"context"
	stdhttp "net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/leaderboard/domain"
	leaderboarduc "virtual-exam-api/internal/leaderboard/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type LeaderboardService interface {
	GetOverview(context.Context, uuid.UUID) (*domain.OverviewResponse, error)
	GetHub(context.Context, uuid.UUID, string, int, int, string, domain.ListFilter) (*domain.HubResponse, error)
	ListAwards(context.Context, uuid.UUID) ([]domain.Award, error)
	GetExamSetLeaderboard(context.Context, uuid.UUID, string, domain.ListFilter) (*domain.ExamSetLeaderboardResponse, error)
}

type ReadLimiter interface {
	Allow(context.Context, uuid.UUID) bool
}

type Handler struct {
	leaderboardUC LeaderboardService
	readLimiter   ReadLimiter
}

func NewHandler(leaderboardUC LeaderboardService, readLimiter ReadLimiter) *Handler {
	return &Handler{leaderboardUC: leaderboardUC, readLimiter: readLimiter}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	g.GET("/leaderboards/overview", h.GetOverview, authMiddleware)
	g.GET("/leaderboards/exam-tracks/:trackCode", h.GetHub, authMiddleware)
	g.GET("/me/leaderboard-awards", h.ListMyAwards, authMiddleware)
	g.GET("/exam-sets/:examSetCode/leaderboard", h.GetExamSetLeaderboard, authMiddleware)
}

func (h *Handler) GetOverview(c echo.Context) error {
	userID, err := h.requireAllowedUser(c)
	if err != nil {
		return response.Error(c, err)
	}
	if err := validateQueryKeys(c); err != nil {
		return response.Error(c, err)
	}

	result, err := h.leaderboardUC.GetOverview(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, stdhttp.StatusOK, result)
}

func (h *Handler) GetHub(c echo.Context) error {
	userID, err := h.requireAllowedUser(c)
	if err != nil {
		return response.Error(c, err)
	}
	year, month, scope, filter, err := parseHubQuery(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.leaderboardUC.GetHub(
		c.Request().Context(),
		userID,
		c.Param("trackCode"),
		year,
		month,
		scope,
		filter,
	)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, stdhttp.StatusOK, result)
}

func (h *Handler) ListMyAwards(c echo.Context) error {
	userID, err := h.requireAllowedUser(c)
	if err != nil {
		return response.Error(c, err)
	}
	if err := validateQueryKeys(c); err != nil {
		return response.Error(c, err)
	}

	result, err := h.leaderboardUC.ListAwards(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, stdhttp.StatusOK, result)
}

func (h *Handler) GetExamSetLeaderboard(c echo.Context) error {
	userID, err := h.requireAllowedUser(c)
	if err != nil {
		return response.Error(c, err)
	}
	if err := validateQueryKeys(c, "page", "limit"); err != nil {
		return response.Error(c, err)
	}
	filter, err := parseListFilter(c, 20)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.leaderboardUC.GetExamSetLeaderboard(
		c.Request().Context(),
		userID,
		c.Param("examSetCode"),
		filter,
	)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, stdhttp.StatusOK, result)
}

func (h *Handler) requireAllowedUser(c echo.Context) (uuid.UUID, error) {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return uuid.Nil, err
	}
	if h.readLimiter != nil && !h.readLimiter.Allow(c.Request().Context(), userID) {
		return uuid.Nil, apperrors.New(
			"RATE_LIMIT_EXCEEDED",
			"คำขอดูอันดับมากเกินไป กรุณาลองใหม่ในอีกสักครู่",
			stdhttp.StatusTooManyRequests,
		)
	}
	return userID, nil
}

func parseHubQuery(c echo.Context) (int, int, string, domain.ListFilter, error) {
	allowed := map[string]struct{}{
		"year": {}, "month": {}, "scope": {}, "page": {}, "limit": {},
	}
	if err := validateQueryKeys(c, allowedKeys(allowed)...); err != nil {
		return 0, 0, "", domain.ListFilter{}, err
	}

	yearText := c.QueryParam("year")
	monthText := c.QueryParam("month")
	if (yearText == "") != (monthText == "") {
		return 0, 0, "", domain.ListFilter{}, apperrors.ValidationError("year and month must be provided together")
	}
	year, month := 0, 0
	var err error
	if yearText != "" {
		year, err = parseBoundedInt(yearText, "year", 1, 9999)
		if err != nil {
			return 0, 0, "", domain.ListFilter{}, err
		}
		month, err = parseBoundedInt(monthText, "month", 1, 12)
		if err != nil {
			return 0, 0, "", domain.ListFilter{}, err
		}
	}

	scope := c.QueryParam("scope")
	if scope == "" {
		scope = leaderboarduc.ScopeTop
	}
	if scope != leaderboarduc.ScopeTop && scope != leaderboarduc.ScopeAroundMe {
		return 0, 0, "", domain.ListFilter{}, apperrors.ValidationError("scope must be top or around_me")
	}

	filter, err := parseListFilter(c, 20)
	if err != nil {
		return 0, 0, "", domain.ListFilter{}, err
	}
	return year, month, scope, filter, nil
}

func parseListFilter(c echo.Context, defaultLimit int) (domain.ListFilter, error) {
	page := 1
	if rawPage := c.QueryParam("page"); rawPage != "" {
		var err error
		page, err = parseBoundedInt(rawPage, "page", 1, 100000)
		if err != nil {
			return domain.ListFilter{}, err
		}
	}
	limit, err := parseOptionalPositiveInt(c.QueryParam("limit"), "limit", defaultLimit)
	if err != nil {
		return domain.ListFilter{}, err
	}
	return domain.ListFilter{Page: page, Limit: limit}, nil
}

func parseOptionalPositiveInt(text, name string, defaultValue int) (int, error) {
	if text == "" {
		return defaultValue, nil
	}
	return parseBoundedInt(text, name, 1, int(^uint(0)>>1))
}

func parseBoundedInt(text, name string, minimum, maximum int) (int, error) {
	value, err := strconv.Atoi(text)
	if err != nil || value < minimum || value > maximum {
		return 0, apperrors.ValidationError(name + " is invalid")
	}
	return value, nil
}

func validateQueryKeys(c echo.Context, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range c.QueryParams() {
		if _, ok := allowedSet[key]; !ok {
			return apperrors.ValidationError("query parameter " + key + " is not supported")
		}
	}
	return nil
}

func allowedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
