package http

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/questionimport/domain"
	importuc "virtual-exam-api/internal/questionimport/usecase"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	uc *importuc.UseCase
}

func NewHandler(uc *importuc.UseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc, adminMiddleware echo.MiddlewareFunc) {
	admin := g.Group("/admin", authMiddleware, adminMiddleware)
	admin.GET("/questions/import/template", h.DownloadTemplate)
	admin.POST("/questions/import/preview", h.Preview)
	admin.POST("/questions/import/confirm", h.Confirm)
}

func (h *Handler) DownloadTemplate(c echo.Context) error {
	data := h.uc.TemplateCSV()
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="question-import-template.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

func (h *Handler) Preview(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, apperrors.New("MISSING_FILE", "กรุณาเลือกไฟล์", 400))
	}

	src, err := file.Open()
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, domain.MaxFileSize+1))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	result, err := h.uc.Preview(c.Request().Context(), adminUserID, file.Filename, data)
	if err != nil {
		return response.Error(c, err)
	}

	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) Confirm(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	var input domain.ImportConfirmInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	result, err := h.uc.Confirm(c.Request().Context(), adminUserID, input)
	if err != nil {
		return response.Error(c, err)
	}

	return response.JSON(c, http.StatusOK, result)
}
