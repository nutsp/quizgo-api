package http

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	esquc "virtual-exam-api/internal/examsetquestion/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	uc *esquc.UseCase
}

func NewHandler(uc *esquc.UseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(admin *echo.Group) {
	admin.GET("/exam-sets/:id/available-questions", h.ListAvailable)
	admin.GET("/exam-sets/:id/questions", h.ListAssigned)
	admin.POST("/exam-sets/:id/questions/bulk", h.BulkAdd)
	admin.POST("/exam-sets/:id/questions", h.AddSingle)
	admin.PUT("/exam-sets/:id/questions/reorder", h.Reorder)
	admin.DELETE("/exam-sets/:id/questions", h.ClearAll)
	admin.DELETE("/exam-sets/:id/questions/:questionId", h.Remove)
}

func (h *Handler) ListAvailable(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	excludeAssigned := c.QueryParam("exclude_assigned") != "false"
	pq := pagination.ParsePagination(c)
	input := esquc.AvailableFilterInput{
		Query:           pq.Q,
		SubjectID:       c.QueryParam("subject_id"),
		TagID:           c.QueryParam("tag_id"),
		Difficulty:      c.QueryParam("difficulty"),
		Status:          c.QueryParam("status"),
		ExcludeAssigned: excludeAssigned,
		Page:            pq.Page,
		Limit:           pq.Limit,
		Sort:            pq.Sort,
		Order:           pq.Order,
	}
	result, err := h.uc.ListAvailable(c.Request().Context(), examSetID, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ListAssigned(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	pq := pagination.ParsePagination(c)
	input := esquc.AssignedFilterInput{
		Query:     pq.Q,
		SubjectID: c.QueryParam("subject_id"),
		TagID:     c.QueryParam("tag_id"),
		Page:      pq.Page,
		Limit:     pq.Limit,
		Sort:      pq.Sort,
		Order:     pq.Order,
	}
	result, err := h.uc.ListAssigned(c.Request().Context(), examSetID, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) BulkAdd(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input esquc.BulkAddInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.uc.BulkAdd(c.Request().Context(), examSetID, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) AddSingle(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var single struct {
		QuestionID string  `json:"question_id"`
		Score      float64 `json:"score"`
	}
	if err := c.Bind(&single); err != nil || single.QuestionID == "" {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.uc.BulkAdd(c.Request().Context(), examSetID, esquc.BulkAddInput{
		QuestionIDs: []string{single.QuestionID},
		Score:       single.Score,
		AppendToEnd: true,
	})
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) Reorder(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input esquc.ReorderInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	if err := h.uc.Reorder(c.Request().Context(), examSetID, input); err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, map[string]string{"status": "reordered"})
}

func (h *Handler) Remove(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	questionID, err := parseUUID(c.Param("questionId"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.uc.Remove(c.Request().Context(), examSetID, questionID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ClearAll(c echo.Context) error {
	examSetID, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input esquc.ClearAllInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.uc.ClearAll(c.Request().Context(), examSetID, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperrors.ErrInvalidUUID
	}
	return id, nil
}

func queryInt(c echo.Context, key string) int {
	v, _ := strconv.Atoi(c.QueryParam(key))
	return v
}
