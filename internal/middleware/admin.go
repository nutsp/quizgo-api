package middleware

import (
	"github.com/labstack/echo/v4"
	userdomain "virtual-exam-api/internal/user/domain"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/response"
)

func AdminOnly() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get(UserRoleKey).(string)
			if role == "" {
				return response.Error(c, apperrors.ErrUnauthorized)
			}
			if role != userdomain.RoleAdmin {
				return response.Error(c, apperrors.ErrForbidden)
			}
			return next(c)
		}
	}
}

func GetUserRole(c echo.Context) string {
	role, _ := c.Get(UserRoleKey).(string)
	return role
}
