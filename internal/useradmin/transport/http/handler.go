package http

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/response"
	useradminrepo "virtual-exam-api/internal/useradmin/repository"
	useradminuc "virtual-exam-api/internal/useradmin/usecase"
	userrepo "virtual-exam-api/internal/user/repository"
)

type Handler struct {
	users   *useradminuc.UseCase
	userRepo userrepo.Repository
}

func NewHandler(users *useradminuc.UseCase, userRepo userrepo.Repository) *Handler {
	return &Handler{users: users, userRepo: userRepo}
}

func (h *Handler) RegisterRoutes(admin *echo.Group) {
	admin.GET("/users", h.List)
	admin.GET("/users/:id", h.Get)
	admin.PUT("/users/:id", h.Update)
	admin.PATCH("/users/:id/status", h.UpdateStatus)
	admin.PATCH("/users/:id/role", h.UpdateRole)
}

func (h *Handler) List(c echo.Context) error {
	pq := pagination.ParsePagination(c)
	filter := useradminrepo.UserAdminFilter{
		Query:  pq.Q,
		Role:   c.QueryParam("role"),
		Status: c.QueryParam("status"),
		Page:   pq.Page,
		Limit:  pq.Limit,
		Sort:   pq.Sort,
		Order:  pq.Order,
	}
	result, err := h.users.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Get(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.users.Get(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) Update(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input useradminuc.UpdateInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	reqCtx, err := h.requestContext(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.users.Update(c.Request().Context(), id, input, reqCtx)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) UpdateStatus(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&body); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	reqCtx, err := h.requestContext(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.users.UpdateStatus(c.Request().Context(), id, body.Status, reqCtx)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) UpdateRole(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&body); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	reqCtx, err := h.requestContext(c)
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.users.UpdateRole(c.Request().Context(), id, body.Role, reqCtx)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) requestContext(c echo.Context) (useradminuc.RequestContext, error) {
	actorID, err := middleware.RequireUserID(c)
	if err != nil {
		return useradminuc.RequestContext{}, err
	}
	actor, err := h.userRepo.FindByID(c.Request().Context(), actorID)
	if err != nil {
		return useradminuc.RequestContext{}, err
	}
	email := ""
	if actor != nil {
		email = actor.Email
	}
	return useradminuc.RequestContext{
		ActorUserID: actorID,
		ActorEmail:  email,
		IPAddress:   c.RealIP(),
		UserAgent:   c.Request().UserAgent(),
	}, nil
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperrors.ErrInvalidUUID
	}
	return id, nil
}
