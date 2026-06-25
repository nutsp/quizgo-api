package response

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
)

type errorBody struct {
	Error apperrors.AppError `json:"error"`
}

type successBody struct {
	Data any `json:"data"`
}

func JSON(c echo.Context, status int, data any) error {
	return c.JSON(status, successBody{Data: data})
}

func Error(c echo.Context, err error) error {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return c.JSON(appErr.HTTPStatus, errorBody{Error: *appErr})
	}
	return c.JSON(http.StatusInternalServerError, errorBody{
		Error: apperrors.AppError{
			Code:       "INTERNAL_ERROR",
			Message:    "เกิดข้อผิดพลาดภายในระบบ",
			HTTPStatus: http.StatusInternalServerError,
		},
	})
}
