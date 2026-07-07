package http

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	attemptuc "virtual-exam-api/internal/examattempt/usecase"
	"virtual-exam-api/internal/examset/domain"
	"virtual-exam-api/internal/examset/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	examSetUC *usecase.ExamSetUseCase
	attemptUC *attemptuc.ExamAttemptUseCase
}

func NewHandler(examSetUC *usecase.ExamSetUseCase, attemptUC *attemptuc.ExamAttemptUseCase) *Handler {
	return &Handler{examSetUC: examSetUC, attemptUC: attemptUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc, optionalAuth echo.MiddlewareFunc) {
	g.GET("/exam-sets/filter-options", h.FilterOptions, optionalAuth)
	g.GET("/exam-sets", h.List, optionalAuth)
	g.GET("/exam-sets/:examSetCode", h.GetByCode, optionalAuth)
	g.GET("/exam-sets/:examSetCode/questions-preview", h.QuestionsPreview, optionalAuth)
	g.POST("/exam-sets/:examSetCode/attempts", h.StartAttempt, authMiddleware)
}

func (h *Handler) FilterOptions(c echo.Context) error {
	userID, _ := middleware.GetUserID(c)
	var uid *uuid.UUID
	if userID != uuid.Nil {
		uid = &userID
	}
	result, err := h.examSetUC.FilterOptions(c.Request().Context(), uid)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) List(c echo.Context) error {
	filter := parseListFilter(c)
	userID, _ := middleware.GetUserID(c)
	var uid *uuid.UUID
	if userID != uuid.Nil {
		uid = &userID
	}
	result, err := h.examSetUC.List(c.Request().Context(), filter, uid)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetByCode(c echo.Context) error {
	code := c.Param("examSetCode")
	userID, _ := middleware.GetUserID(c)
	var uid *uuid.UUID
	if userID != uuid.Nil {
		uid = &userID
	}
	result, err := h.examSetUC.GetByCode(c.Request().Context(), code, uid)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) QuestionsPreview(c echo.Context) error {
	code := c.Param("examSetCode")
	userID, _ := middleware.GetUserID(c)
	var uid *uuid.UUID
	if userID != uuid.Nil {
		uid = &userID
	}
	result, err := h.examSetUC.QuestionsPreview(c.Request().Context(), code, uid)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) StartAttempt(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	code := c.Param("examSetCode")
	result, err := h.attemptUC.Start(c.Request().Context(), userID, code)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func parseListFilter(c echo.Context) domain.ListFilter {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	trackParam := firstNonEmpty(c.QueryParam("track"), c.QueryParam("track_code"))
	return domain.ListFilter{
		Query:         c.QueryParam("q"),
		TrackCode:     trackParam,
		TrackCodes:    parseCSVParam(trackParam),
		SubjectCodes:  parseCSVParam(c.QueryParam("subject")),
		QuestionTypes: parseCSVParam(c.QueryParam("question_type")),
		AccessType:    c.QueryParam("access_type"),
		AccessTypes:   parseCSVParam(c.QueryParam("access_type")),
		Difficulty:    c.QueryParam("difficulty"),
		Difficulties:  parseCSVParam(c.QueryParam("difficulty")),
		Mode:          c.QueryParam("mode"),
		Modes:         parseCSVParam(c.QueryParam("mode")),
		Statuses:      parseCSVParam(c.QueryParam("status")),
		Sort:          c.QueryParam("sort"),
		Page:          page,
		Limit:         limit,
	}
}

func parseCSVParam(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
