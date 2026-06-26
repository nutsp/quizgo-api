package http

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/leaderboard/domain"
	"virtual-exam-api/internal/leaderboard/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	leaderboardUC *usecase.LeaderboardUseCase
}

func NewHandler(leaderboardUC *usecase.LeaderboardUseCase) *Handler {
	return &Handler{leaderboardUC: leaderboardUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	g.GET("/exam-sets/:examSetCode/leaderboard", h.GetExamSetLeaderboard, authMiddleware)
	g.GET("/exam-tracks/:trackCode/leaderboard", h.GetExamTrackLeaderboard, authMiddleware)
}

func (h *Handler) GetExamSetLeaderboard(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.leaderboardUC.GetExamSetLeaderboard(
		c.Request().Context(),
		userID,
		c.Param("examSetCode"),
		parseFilter(c),
	)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetExamTrackLeaderboard(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.leaderboardUC.GetExamTrackLeaderboard(
		c.Request().Context(),
		userID,
		c.Param("trackCode"),
		parseFilter(c),
	)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func parseFilter(c echo.Context) domain.ListFilter {
	return domain.ListFilter{
		Page:  parseIntDefault(c.QueryParam("page"), 1),
		Limit: parseIntDefault(c.QueryParam("limit"), 20),
	}
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return def
	}
	return v
}
