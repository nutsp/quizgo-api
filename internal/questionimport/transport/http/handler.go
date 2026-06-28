package http

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/common/pagination"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/questionimport/domain"
	importuc "virtual-exam-api/internal/questionimport/usecase"
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
	admin.POST("/questions/import/confirm", h.Confirm)
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
				"failed_rows":          result.FailedRows,
			},
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
		})
	}

	return response.JSON(c, http.StatusOK, result)
}
