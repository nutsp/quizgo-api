package http

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/examset/domain"
	"virtual-exam-api/internal/examtrack/usecase"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	trackUC *usecase.ExamTrackUseCase
}

func NewHandler(trackUC *usecase.ExamTrackUseCase) *Handler {
	return &Handler{trackUC: trackUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/exam-tracks", h.List)
	g.GET("/exam-tracks/:trackCode", h.GetByCode)
	g.GET("/exam-tracks/:trackCode/exam-sets", h.ListExamSets)
}

func (h *Handler) List(c echo.Context) error {
	result, err := h.trackUC.List(c.Request().Context())
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetByCode(c echo.Context) error {
	code := c.Param("trackCode")
	result, err := h.trackUC.GetByCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ListExamSets(c echo.Context) error {
	code := c.Param("trackCode")
	filter := parseListFilter(c)
	result, err := h.trackUC.ListExamSets(c.Request().Context(), code, filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
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

