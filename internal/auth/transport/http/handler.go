package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	authdomain "virtual-exam-api/internal/auth/domain"
	"virtual-exam-api/internal/auth/oauth"
	"virtual-exam-api/internal/auth/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	authUC    *usecase.AuthUseCase
	oauthSvc  *oauth.Service
}

func NewHandler(authUC *usecase.AuthUseCase, oauthSvc *oauth.Service) *Handler {
	return &Handler{authUC: authUC, oauthSvc: oauthSvc}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc) {
	auth := g.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.GET("/me", h.Me, authMiddleware)

	oauthGroup := auth.Group("/oauth")
	oauthGroup.GET("/google/login", h.GoogleLogin)
	oauthGroup.GET("/google/callback", h.GoogleCallback)
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

func (h *Handler) GoogleLogin(c echo.Context) error {
	redirect := c.QueryParam("redirect")
	url, err := h.oauthSvc.BeginGoogleLogin(c.Response(), redirect)
	if err != nil {
		return c.Redirect(http.StatusFound, oauth.BuildFrontendErrorURL(h.oauthSvc.FrontendURL()))
	}
	return c.Redirect(http.StatusFound, url)
}

func (h *Handler) GoogleCallback(c echo.Context) error {
	code := c.QueryParam("code")
	state := c.QueryParam("state")

	target, err := h.oauthSvc.HandleGoogleCallback(c.Response(), c.Request(), code, state)
	if err != nil {
		return c.Redirect(http.StatusFound, oauth.BuildFrontendErrorURL(h.oauthSvc.FrontendURL()))
	}
	return c.Redirect(http.StatusFound, target)
}
