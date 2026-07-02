package http

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/media/storage"
	"virtual-exam-api/internal/response"
)

type MediaHandler struct {
	store *storage.LocalStorage
}

func NewMediaHandler(store *storage.LocalStorage) *MediaHandler {
	return &MediaHandler{store: store}
}

func (h *MediaHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/media/upload", h.Upload)
}

func (h *MediaHandler) Upload(c echo.Context) error {
	if h.store == nil {
		return response.Error(c, apperrors.New("STORAGE_UNAVAILABLE", "ระบบอัปโหลดไม่พร้อมใช้งาน", 503))
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

	data, err := io.ReadAll(io.LimitReader(src, storage.MaxImageSize+1))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	url, err := h.store.SaveImage("questions/uploads", file.Filename, data)
	if err != nil {
		return response.Error(c, apperrors.ValidationError(err.Error()))
	}

	return response.JSON(c, http.StatusOK, map[string]string{
		"url": url,
	})
}
