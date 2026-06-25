package http

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/home/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	homeUC *usecase.HomeUseCase
}

func NewHandler(homeUC *usecase.HomeUseCase) *Handler {
	return &Handler{homeUC: homeUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, optionalAuth echo.MiddlewareFunc) {
	g.GET("/home", h.GetHome, optionalAuth)
}

func (h *Handler) GetHome(c echo.Context) error {
	var userID *uuid.UUID
	if id, ok := middleware.GetUserID(c); ok {
		userID = &id
	}

	result, err := h.homeUC.GetHome(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}
