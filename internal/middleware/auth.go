package middleware

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/auth/usecase"
	"virtual-exam-api/internal/response"
)

const UserIDKey = "userID"
const UserRoleKey = "userRole"

func JWTAuth(authUC *usecase.AuthUseCase) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
			if authHeader == "" {
				return response.Error(c, apperrors.ErrUnauthorized)
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return response.Error(c, apperrors.ErrUnauthorized)
			}

			claims, err := authUC.ParseToken(parts[1])
			if err != nil {
				return response.Error(c, apperrors.ErrUnauthorized)
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				return response.Error(c, apperrors.ErrUnauthorized)
			}

			c.Set(UserIDKey, userID)
			c.Set(UserRoleKey, claims.Role)
			return next(c)
		}
	}
}

func OptionalJWTAuth(authUC *usecase.AuthUseCase) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					if claims, err := authUC.ParseToken(parts[1]); err == nil {
						if userID, err := uuid.Parse(claims.UserID); err == nil {
							c.Set(UserIDKey, userID)
							c.Set(UserRoleKey, claims.Role)
						}
					}
				}
			}
			return next(c)
		}
	}
}

func GetUserID(c echo.Context) (uuid.UUID, bool) {
	v, ok := c.Get(UserIDKey).(uuid.UUID)
	return v, ok
}

func RequireUserID(c echo.Context) (uuid.UUID, error) {
	userID, ok := GetUserID(c)
	if !ok {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return userID, nil
}
