package http

import (
	"github.com/labstack/echo/v4"
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
	result, err := h.entitlements.ListMyExams(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}
