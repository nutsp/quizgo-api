package http

import (
	"github.com/labstack/echo/v4"
	authdomain "virtual-exam-api/internal/auth/domain"
	"virtual-exam-api/internal/auth/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	authUC *usecase.AuthUseCase
}

func NewHandler(authUC *usecase.AuthUseCase) *Handler {
	return &Handler{authUC: authUC}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	auth := g.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.GET("/me", h.Me, authMiddleware)
}

func (h *Handler) Register(c echo.Context) error {
	var req authdomain.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, err)
	}

	result, err := h.authUC.Register(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) Login(c echo.Context) error {
	var req authdomain.LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, err)
	}

	result, err := h.authUC.Login(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Me(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	result, err := h.authUC.Me(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}
