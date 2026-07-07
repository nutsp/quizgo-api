package http

import (
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/questionimport/domain"
	importuc "virtual-exam-api/internal/questionimport/usecase"
	"virtual-exam-api/internal/questionimport/zipimages"
	"virtual-exam-api/internal/response"
	userrepo "virtual-exam-api/internal/user/repository"
)

type Handler struct {
	uc    *importuc.UseCase
	audit *audituc.Logger
	users userrepo.Repository
}

func NewHandler(uc *importuc.UseCase, audit *audituc.Logger, users userrepo.Repository) *Handler {
	return &Handler{uc: uc, audit: audit, users: users}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc, adminMiddleware echo.MiddlewareFunc) {
	admin := g.Group("/admin", authMiddleware, adminMiddleware)
	admin.GET("/questions/import/template", h.DownloadTemplate)
	admin.GET("/questions/import/jobs", h.ListJobs)
	admin.POST("/questions/import/preview", h.Preview)
	admin.GET("/questions/import/preview/:importId/rows", h.ListPreviewRows)
	admin.POST("/questions/import/confirm", h.Confirm)
	admin.GET("/question-import-batches", h.ListJobs)
	admin.POST("/question-import-batches", h.CreateBatch)
	admin.GET("/question-import-batches/:id", h.GetBatch)
	admin.GET("/question-import-batches/:id/items", h.ListBatchItems)
	admin.PUT("/question-import-batches/:id/items/:itemId", h.UpdateBatchItem)
	admin.POST("/question-import-batches/:id/approve", h.ApproveBatch)
	admin.POST("/question-import-batches/:id/reject", h.RejectBatch)
	admin.GET("/question-import-batches/:id/error-report", h.DownloadErrorReport)
}

func (h *Handler) DownloadTemplate(c echo.Context) error {
	data := h.uc.TemplateCSV()
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="question-import-template.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

func (h *Handler) ListJobs(c echo.Context) error {
	pq := pagination.ParsePagination(c)
	input := importuc.ImportJobListFilter{
		Query:    pq.Q,
		Status:   c.QueryParam("status"),
		DateFrom: c.QueryParam("date_from"),
		DateTo:   c.QueryParam("date_to"),
		Page:     pq.Page,
		Limit:    pq.Limit,
		Sort:     pq.Sort,
		Order:    pq.Order,
	}
	result, err := h.uc.ListJobs(c.Request().Context(), input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) Preview(c echo.Context) error {
	return h.createBatchFromUpload(c)
}

func (h *Handler) CreateBatch(c echo.Context) error {
	return h.createBatchFromUpload(c)
}

func (h *Handler) createBatchFromUpload(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		file, err = c.FormFile("question_file")
		if err != nil {
			return response.Error(c, apperrors.New("MISSING_FILE", "กรุณาเลือกไฟล์", 400))
		}
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

	var zipData []byte
	if zipFile, err := c.FormFile("images_zip"); err == nil && zipFile != nil {
		zipSrc, err := zipFile.Open()
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
		zipData, err = io.ReadAll(io.LimitReader(zipSrc, zipimages.MaxZipSize+1))
		zipSrc.Close()
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
	} else if zipFile, err := c.FormFile("image_zip"); err == nil && zipFile != nil {
		zipSrc, err := zipFile.Open()
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
		zipData, err = io.ReadAll(io.LimitReader(zipSrc, zipimages.MaxZipSize+1))
		zipSrc.Close()
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidInput)
		}
	}

	result, err := h.uc.Preview(c.Request().Context(), adminUserID, file.Filename, data, zipData)
	if err != nil {
		return response.Error(c, err)
	}

	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) GetBatch(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.uc.GetJob(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) ListBatchItems(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	importID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	resultFilter := c.QueryParam("result")
	status := ""
	switch resultFilter {
	case "valid":
		status = "valid"
	case "invalid":
		status = "error"
	}

	pq := pagination.ParsePagination(c)
	input := importuc.PreviewRowListInput{
		ImportID: importID,
		Page:     pq.Page,
		Limit:    pq.Limit,
		Status:   status,
		Search:   c.QueryParam("q"),
	}

	result, err := h.uc.ListPreviewRows(c.Request().Context(), adminUserID, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) UpdateBatchItem(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	batchID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	var body importuc.UpdateImportItemInput
	if err := c.Bind(&body); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	body.BatchID = batchID
	body.ItemID = itemID

	result, err := h.uc.UpdateItem(c.Request().Context(), adminUserID, body)
	if err != nil {
		return response.Error(c, err)
	}

	if h.audit != nil {
		email := ""
		if actor, err := h.users.FindByID(c.Request().Context(), adminUserID); err == nil && actor != nil {
			email = actor.Email
		}
		h.audit.Log(c.Request().Context(), audituc.LogInput{
			ActorUserID:  &adminUserID,
			ActorEmail:   email,
			Action:       "question_import.item_update",
			ResourceType: "question_import_item",
			ResourceID:   &itemID,
			ResourceName: "question import item",
			AfterData: map[string]any{
				"batch_id": batchID.String(),
				"is_valid": result.IsValid,
			},
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
		})
	}

	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) ApproveBatch(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	importID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.uc.Approve(c.Request().Context(), adminUserID, importID)
	if err != nil {
		return response.Error(c, err)
	}
	if h.audit != nil {
		email := ""
		if actor, err := h.users.FindByID(c.Request().Context(), adminUserID); err == nil && actor != nil {
			email = actor.Email
		}
		h.audit.Log(c.Request().Context(), audituc.LogInput{
			ActorUserID:  &adminUserID,
			ActorEmail:   email,
			Action:       "question_import.approve",
			ResourceType: "question_import_job",
			ResourceID:   &importID,
			ResourceName: "question import",
			AfterData: map[string]any{
				"imported_questions": result.ImportedQuestions,
				"skipped_rows":       result.SkippedRows,
				"failed_rows":        result.FailedRows,
			},
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
		})
	}
	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) RejectBatch(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	importID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind(&body)
	result, err := h.uc.Reject(c.Request().Context(), adminUserID, domain.ImportRejectInput{
		ImportID: importID,
		Reason:   body.Reason,
	})
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) DownloadErrorReport(c echo.Context) error {
	importID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	data, err := h.uc.ErrorReport(c.Request().Context(), importID)
	if err != nil {
		return response.Error(c, err)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="question-import-error-report.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

func (h *Handler) ListPreviewRows(c echo.Context) error {
	adminUserID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}

	importID, err := uuid.Parse(c.Param("importId"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}

	pq := pagination.ParsePagination(c)
	input := importuc.PreviewRowListInput{
		ImportID:     importID,
		Page:         pq.Page,
		Limit:        pq.Limit,
		Status:       c.QueryParam("status"),
		Search:       c.QueryParam("search"),
		SubjectCode:  c.QueryParam("subject_code"),
		QuestionType: c.QueryParam("question_type"),
	}

	result, err := h.uc.ListPreviewRows(c.Request().Context(), adminUserID, input)
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

	if h.audit != nil {
		email := ""
		if actor, err := h.users.FindByID(c.Request().Context(), adminUserID); err == nil && actor != nil {
			email = actor.Email
		}
		importID := input.ImportID
		h.audit.Log(c.Request().Context(), audituc.LogInput{
			ActorUserID:  &adminUserID,
			ActorEmail:   email,
			Action:       "question.import",
			ResourceType: "question_import_job",
			ResourceID:   &importID,
			ResourceName: "question import",
			AfterData: map[string]any{
				"imported_questions": result.ImportedQuestions,
				"skipped_rows":       result.SkippedRows,
				"failed_rows":        result.FailedRows,
			},
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
		})
	}

	return response.JSON(c, http.StatusOK, result)
}
