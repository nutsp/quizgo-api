package http

import (
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/middleware"
	profiledomain "virtual-exam-api/internal/profile/domain"
	"virtual-exam-api/internal/profile/usecase"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	profileUC *usecase.ProfileUseCase
}

func NewHandler(profileUC *usecase.ProfileUseCase) *Handler {
	return &Handler{profileUC: profileUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	me := g.Group("/me", authMiddleware)
	me.GET("/profile", h.GetProfile)
	me.PUT("/profile", h.UpdateProfile)
}

func (h *Handler) GetProfile(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.profileUC.GetProfile(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) UpdateProfile(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	var req profiledomain.UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	result, err := h.profileUC.UpdateProfile(c.Request().Context(), userID, req)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}
