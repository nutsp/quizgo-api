package http

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examattempt/domain"
	"virtual-exam-api/internal/examattempt/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	attemptUC *usecase.ExamAttemptUseCase
}

func NewHandler(attemptUC *usecase.ExamAttemptUseCase) *Handler {
	return &Handler{attemptUC: attemptUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	attempts := g.Group("/attempts", authMiddleware)
	attempts.GET("/:attemptId", h.Get)
	attempts.PUT("/:attemptId/answers/:questionNo", h.SaveAnswer)
	attempts.DELETE("/:attemptId/answers/:questionNo", h.ClearAnswer)
	attempts.POST("/:attemptId/submit", h.Submit)
	attempts.GET("/:attemptId/result", h.GetResult)
	attempts.GET("/:attemptId/review", h.GetReview)
}

func (h *Handler) Get(c echo.Context) error {
	userID, attemptID, err := parseAttempt(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.attemptUC.Get(c.Request().Context(), userID, attemptID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) SaveAnswer(c echo.Context) error {
	userID, attemptID, err := parseAttempt(c)
	if err != nil {
		return response.Error(c, err)
	}
	questionNo, err := strconv.Atoi(c.Param("questionNo"))
	if err != nil || questionNo < 1 {
		return response.Error(c, apperrors.ErrQuestionNotFound)
	}

	var req domain.SaveAnswerRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	result, err := h.attemptUC.SaveAnswer(c.Request().Context(), userID, attemptID, questionNo, req)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ClearAnswer(c echo.Context) error {
	userID, attemptID, err := parseAttempt(c)
	if err != nil {
		return response.Error(c, err)
	}
	questionNo, err := strconv.Atoi(c.Param("questionNo"))
	if err != nil || questionNo < 1 {
		return response.Error(c, apperrors.ErrQuestionNotFound)
	}

	result, err := h.attemptUC.ClearAnswer(c.Request().Context(), userID, attemptID, questionNo)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Submit(c echo.Context) error {
	userID, attemptID, err := parseAttempt(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.attemptUC.Submit(c.Request().Context(), userID, attemptID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetResult(c echo.Context) error {
	userID, attemptID, err := parseAttempt(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.attemptUC.GetResult(c.Request().Context(), userID, attemptID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetReview(c echo.Context) error {
	userID, attemptID, err := parseAttempt(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.attemptUC.GetReview(c.Request().Context(), userID, attemptID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func parseAttempt(c echo.Context) (uuid.UUID, uuid.UUID, error) {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	attemptID, err := uuid.Parse(c.Param("attemptId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apperrors.ErrInvalidUUID
	}
	return userID, attemptID, nil
}
