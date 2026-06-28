package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	authdomain "virtual-exam-api/internal/auth/domain"
	"virtual-exam-api/internal/auth/oauth"
	"virtual-exam-api/internal/auth/usecase"
	accessdomain "virtual-exam-api/internal/accesslog/domain"
	accessuc "virtual-exam-api/internal/accesslog/usecase"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
	userrepo "virtual-exam-api/internal/user/repository"
)

type Handler struct {
	authUC      *usecase.AuthUseCase
	oauthSvc    *oauth.Service
	accessLog   *accessuc.Logger
	users       userrepo.Repository
}

func NewHandler(authUC *usecase.AuthUseCase, oauthSvc *oauth.Service, accessLog *accessuc.Logger, users userrepo.Repository) *Handler {
	return &Handler{
		authUC:    authUC,
		oauthSvc:  oauthSvc,
		accessLog: accessLog,
		users:     users,
	}
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

	ctx := c.Request().Context()
	email := strings.ToLower(strings.TrimSpace(req.Email))
	result, err := h.authUC.Login(ctx, req)
	if err != nil {
		h.logLoginFailure(c, email, err)
		return response.Error(c, err)
	}

	userID, _ := uuid.Parse(result.User.ID)
	h.accessLog.Log(ctx, accessuc.LogInput{
		UserID:    &userID,
		Email:     result.User.Email,
		EventType: accessdomain.EventLoginSuccess,
		Success:   true,
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	})
	return response.JSON(c, 200, result)
}

func (h *Handler) logLoginFailure(c echo.Context, email string, err error) {
	if h.accessLog == nil {
		return
	}
	var userID *uuid.UUID
	if email != "" {
		user, findErr := h.users.FindByEmail(c.Request().Context(), email)
		if findErr == nil && user != nil {
			userID = &user.ID
		}
	}
	eventType := accessdomain.EventLoginFailed
	message := ""
	if errors.Is(err, apperrors.ErrAccountSuspended) {
		eventType = accessdomain.EventAccountSuspendedLoginBlocked
		message = apperrors.ErrAccountSuspended.Message
	}
	h.accessLog.Log(c.Request().Context(), accessuc.LogInput{
		UserID:    userID,
		Email:     email,
		EventType: eventType,
		Success:   false,
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
		Message:   message,
	})
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
	ctx := c.Request().Context()

	target, loginResp, err := h.oauthSvc.HandleGoogleCallback(c.Response(), c.Request(), code, state)
	if err != nil || loginResp == nil {
		h.accessLog.Log(ctx, accessuc.LogInput{
			EventType: accessdomain.EventOAuthLoginFailed,
			Success:   false,
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
		})
		return c.Redirect(http.StatusFound, target)
	}

	userID, _ := uuid.Parse(loginResp.User.ID)
	h.accessLog.Log(ctx, accessuc.LogInput{
		UserID:    &userID,
		Email:     loginResp.User.Email,
		EventType: accessdomain.EventOAuthLoginSuccess,
		Success:   true,
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	})
	return c.Redirect(http.StatusFound, target)
}
