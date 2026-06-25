package http

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
	"virtual-exam-api/internal/result/domain"
	"virtual-exam-api/internal/result/usecase"
)

type Handler struct {
	resultUC *usecase.ResultUseCase
}

func NewHandler(resultUC *usecase.ResultUseCase) *Handler {
	return &Handler{resultUC: resultUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	results := g.Group("/me/results", authMiddleware)
	results.GET("/summary", h.GetMyResultsSummary)
	results.GET("/exam-tracks", h.GetMyExamTrackResults)
	results.GET("/exam-tracks/:trackCode", h.GetMyExamTrackResultDetail)
	results.GET("", h.ListMyAttemptResults)
	results.GET("/exam-sets/:examSetCode", h.GetMyExamSetResultDetail)
}

func (h *Handler) GetMyResultsSummary(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.resultUC.GetMyResultsSummary(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetMyExamTrackResults(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.resultUC.GetMyExamTrackResults(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetMyExamTrackResultDetail(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.resultUC.GetMyExamTrackResultDetail(c.Request().Context(), userID, c.Param("trackCode"))
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ListMyAttemptResults(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	filter := domain.AttemptHistoryFilter{
		ExamTrackCode: c.QueryParam("exam_track_code"),
		ExamSetCode:   c.QueryParam("exam_set_code"),
		Status:        c.QueryParam("status"),
		Page:          parseIntDefault(c.QueryParam("page"), 1),
		Limit:         parseIntDefault(c.QueryParam("limit"), 20),
	}

	if df := c.QueryParam("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			filter.DateFrom = &t
		}
	}
	if dt := c.QueryParam("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			filter.DateTo = &t
		}
	}

	result, err := h.resultUC.ListMyAttemptResults(c.Request().Context(), userID, filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetMyExamSetResultDetail(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.resultUC.GetMyExamSetResultDetail(c.Request().Context(), userID, c.Param("examSetCode"))
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
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
