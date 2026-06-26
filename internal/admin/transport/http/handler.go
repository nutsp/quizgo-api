package http

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	dashboarduc "virtual-exam-api/internal/admin/dashboard/usecase"
	"virtual-exam-api/internal/apperrors"
	examsetuc "virtual-exam-api/internal/examset/usecase"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	trackuc "virtual-exam-api/internal/examtrack/usecase"
	questionrepo "virtual-exam-api/internal/question/repository"
	questionuc "virtual-exam-api/internal/question/usecase"
	"virtual-exam-api/internal/response"
	subjectrepo "virtual-exam-api/internal/subject/repository"
	subjectuc "virtual-exam-api/internal/subject/usecase"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	esqhttp "virtual-exam-api/internal/examsetquestion/transport/http"
)

type Handler struct {
	dashboard        *dashboarduc.DashboardUseCase
	tracks           *trackuc.AdminUseCase
	sets             *examsetuc.AdminUseCase
	subjects         *subjectuc.SubjectUseCase
	questions        *questionuc.AdminUseCase
	examSetQuestions *esqhttp.Handler
}

func NewHandler(
	dashboard *dashboarduc.DashboardUseCase,
	tracks *trackuc.AdminUseCase,
	sets *examsetuc.AdminUseCase,
	subjects *subjectuc.SubjectUseCase,
	questions *questionuc.AdminUseCase,
	examSetQuestions *esqhttp.Handler,
) *Handler {
	return &Handler{
		dashboard:        dashboard,
		tracks:           tracks,
		sets:             sets,
		subjects:         subjects,
		questions:        questions,
		examSetQuestions: examSetQuestions,
	}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMiddleware echo.MiddlewareFunc, adminMiddleware echo.MiddlewareFunc) {
	admin := g.Group("/admin", authMiddleware, adminMiddleware)
	admin.GET("/dashboard", h.Dashboard)

	admin.GET("/exam-tracks", h.ListTracks)
	admin.POST("/exam-tracks", h.CreateTrack)
	admin.GET("/exam-tracks/:id", h.GetTrack)
	admin.PUT("/exam-tracks/:id", h.UpdateTrack)
	admin.DELETE("/exam-tracks/:id", h.DeleteTrack)

	admin.GET("/exam-sets", h.ListSets)
	admin.POST("/exam-sets", h.CreateSet)
	admin.GET("/exam-sets/:id/readiness", h.GetSetReadiness)
	admin.GET("/exam-sets/:id/preview", h.GetSetPreview)
	admin.POST("/exam-sets/:id/publish", h.PublishSet)
	admin.POST("/exam-sets/:id/unpublish", h.UnpublishSet)
	admin.POST("/exam-sets/:id/archive", h.ArchiveSet)
	admin.GET("/exam-sets/:id", h.GetSet)
	admin.PUT("/exam-sets/:id", h.UpdateSet)
	admin.DELETE("/exam-sets/:id", h.DeleteSet)

	if h.examSetQuestions != nil {
		h.examSetQuestions.RegisterRoutes(admin)
	}

	admin.GET("/subjects", h.ListSubjects)
	admin.POST("/subjects", h.CreateSubject)
	admin.GET("/subjects/:id", h.GetSubject)
	admin.PUT("/subjects/:id", h.UpdateSubject)
	admin.DELETE("/subjects/:id", h.DeleteSubject)

	admin.GET("/questions", h.ListQuestions)
	admin.POST("/questions", h.CreateQuestion)
	admin.GET("/questions/:id", h.GetQuestion)
	admin.PUT("/questions/:id", h.UpdateQuestion)
	admin.DELETE("/questions/:id", h.DeleteQuestion)
}

func (h *Handler) Dashboard(c echo.Context) error {
	result, err := h.dashboard.Get(c.Request().Context())
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ListTracks(c echo.Context) error {
	filter := trackrepo.AdminFilter{
		Query: c.QueryParam("q"),
		Page:  queryInt(c, "page"),
		Limit: queryInt(c, "limit"),
	}
	if v := c.QueryParam("is_active"); v != "" {
		active := v == "true"
		filter.IsActive = &active
	}
	result, err := h.tracks.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetTrack(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.tracks.Get(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) CreateTrack(c echo.Context) error {
	var input trackuc.CreateTrackInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.tracks.Create(c.Request().Context(), input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) UpdateTrack(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input trackuc.UpdateTrackInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.tracks.Update(c.Request().Context(), id, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) DeleteTrack(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	deactivated, err := h.tracks.Delete(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, map[string]any{"deactivated": deactivated})
}

func (h *Handler) ListSets(c echo.Context) error {
	filter := examsetrepo.AdminFilter{
		Query:      c.QueryParam("q"),
		AccessType: c.QueryParam("access_type"),
		Difficulty: c.QueryParam("difficulty"),
		Mode:       c.QueryParam("mode"),
		Page:       queryInt(c, "page"),
		Limit:      queryInt(c, "limit"),
	}
	if trackID := c.QueryParam("exam_track_id"); trackID != "" {
		id, err := uuid.Parse(trackID)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidUUID)
		}
		filter.TrackID = id
	}
	if v := c.QueryParam("is_active"); v != "" {
		active := v == "true"
		filter.IsActive = &active
	}
	result, err := h.sets.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetSet(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.sets.Get(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) CreateSet(c echo.Context) error {
	var input examsetuc.CreateSetInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.sets.Create(c.Request().Context(), input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) UpdateSet(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input examsetuc.UpdateSetInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.sets.Update(c.Request().Context(), id, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) DeleteSet(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	deactivated, err := h.sets.Delete(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, map[string]any{"deactivated": deactivated})
}

func (h *Handler) GetSetReadiness(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.sets.CheckReadiness(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetSetPreview(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.sets.GetPreview(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) PublishSet(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.sets.Publish(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) UnpublishSet(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.sets.Unpublish(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ArchiveSet(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.sets.Archive(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) ListSubjects(c echo.Context) error {
	filter := subjectrepo.SubjectAdminFilter{
		Query: c.QueryParam("q"),
		Page:  queryInt(c, "page"),
		Limit: queryInt(c, "limit"),
	}
	result, err := h.subjects.List(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetSubject(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.subjects.Get(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) CreateSubject(c echo.Context) error {
	var input subjectuc.SubjectInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.subjects.Create(c.Request().Context(), input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) UpdateSubject(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input subjectuc.SubjectInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.subjects.Update(c.Request().Context(), id, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) DeleteSubject(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	if err := h.subjects.Delete(c.Request().Context(), id); err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, map[string]string{"status": "deleted"})
}

func (h *Handler) ListQuestions(c echo.Context) error {
	filter := questionrepo.QuestionAdminFilter{
		Query:      c.QueryParam("q"),
		Difficulty: c.QueryParam("difficulty"),
		Status:     c.QueryParam("status"),
		Page:       queryInt(c, "page"),
		Limit:      queryInt(c, "limit"),
	}
	if sid := c.QueryParam("subject_id"); sid != "" {
		id, err := uuid.Parse(sid)
		if err != nil {
			return response.Error(c, apperrors.ErrInvalidUUID)
		}
		filter.SubjectID = id
	}
	result, err := h.questions.ListQuestions(c.Request().Context(), filter)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) GetQuestion(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	result, err := h.questions.GetQuestion(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) CreateQuestion(c echo.Context) error {
	var input questionuc.QuestionInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.questions.CreateQuestion(c.Request().Context(), input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 201, result)
}

func (h *Handler) UpdateQuestion(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	var input questionuc.QuestionInput
	if err := c.Bind(&input); err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	result, err := h.questions.UpdateQuestion(c.Request().Context(), id, input)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, result)
}

func (h *Handler) DeleteQuestion(c echo.Context) error {
	id, err := parseUUID(c.Param("id"))
	if err != nil {
		return response.Error(c, err)
	}
	archived, err := h.questions.DeleteQuestion(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, 200, map[string]any{"archived": archived})
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperrors.ErrInvalidUUID
	}
	return id, nil
}

func queryInt(c echo.Context, key string) int {
	v, _ := strconv.Atoi(c.QueryParam(key))
	return v
}
