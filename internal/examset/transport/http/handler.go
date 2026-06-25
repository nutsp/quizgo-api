package http

import (
	"strconv"

	"github.com/labstack/echo/v4"
	attemptuc "virtual-exam-api/internal/examattempt/usecase"
	"virtual-exam-api/internal/examset/domain"
	"virtual-exam-api/internal/examset/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	examSetUC  *usecase.ExamSetUseCase
	attemptUC  *attemptuc.ExamAttemptUseCase
}

func NewHandler(examSetUC *usecase.ExamSetUseCase, attemptUC *attemptuc.ExamAttemptUseCase) *Handler {
	return &Handler{examSetUC: examSetUC, attemptUC: attemptUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	g.GET("/exam-sets", h.List)
	g.GET("/exam-sets/:examSetCode", h.GetByCode)
	g.GET("/exam-sets/:examSetCode/questions-preview", h.QuestionsPreview)
	g.POST("/exam-sets/:examSetCode/attempts", h.StartAttempt, authMiddleware)
}

func (h *Handler) List(c echo.Context) error {
	filter := parseListFilter(c)
	result, err := h.examSetUC.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetByCode(c echo.Context) error {
	code := c.Param("examSetCode")
	result, err := h.examSetUC.GetByCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) QuestionsPreview(c echo.Context) error {
	code := c.Param("examSetCode")
	result, err := h.examSetUC.QuestionsPreview(c.Request().Context(), code)
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
	return domain.ListFilter{
		Query:      c.QueryParam("q"),
		AccessType: c.QueryParam("access_type"),
		Difficulty: c.QueryParam("difficulty"),
		Mode:       c.QueryParam("mode"),
		Page:       page,
		Limit:      limit,
	}
}
