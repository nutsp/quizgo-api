package http

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/entitlement/domain"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

func (h *Handler) RegisterUserRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	me := g.Group("/me", authMiddleware)
	me.GET("/exams", h.ListMyExams)
}

func (h *Handler) ListMyExams(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	tab := c.QueryParam("tab")

	result, err := h.entitlements.ListMyExams(c.Request().Context(), userID, domain.MyExamsListParams{
		Page:  page,
		Limit: limit,
		Tab:   tab,
	})
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}
