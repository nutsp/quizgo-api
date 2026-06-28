package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/common/pagination"
	esdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	esqdomain "virtual-exam-api/internal/examsetquestion/domain"
	esqrepo "virtual-exam-api/internal/examsetquestion/repository"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type UseCase struct {
	repo        esqrepo.Repository
	questions   questionrepo.QuestionAdminRepository
	sets        examsetrepo.Repository
	setAdmin    examsetrepo.AdminRepository
	trackAdmin  trackrepo.AdminRepository
	invalidator *cache.Invalidator
}

func NewUseCase(
	repo esqrepo.Repository,
	questions questionrepo.QuestionAdminRepository,
	sets examsetrepo.Repository,
	setAdmin examsetrepo.AdminRepository,
	trackAdmin trackrepo.AdminRepository,
	invalidator *cache.Invalidator,
) *UseCase {
	return &UseCase{
		repo:        repo,
		questions:   questions,
		sets:        sets,
		setAdmin:    setAdmin,
		trackAdmin:  trackAdmin,
		invalidator: invalidator,
	}
}

type AvailableFilterInput struct {
	Query           string
	SubjectID       string
	TagID           string
	Difficulty      string
	Status          string
	ExcludeAssigned bool
	Page            int
	Limit           int
	Sort            string
	Order           string
}

type AssignedFilterInput struct {
	Query     string
	SubjectID string
	TagID     string
	Page      int
	Limit     int
	Sort      string
	Order     string
}

type TagSummaryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Code  string `json:"code"`
	Color string `json:"color,omitempty"`
}

type AvailableQuestionResponse struct {
	ID               string      `json:"id"`
	QuestionText     string      `json:"question_text"`
	Subject          *SubjectDTO `json:"subject,omitempty"`
	Difficulty       string      `json:"difficulty"`
	Status           string      `json:"status"`
	CorrectChoiceKey string      `json:"correct_choice_key,omitempty"`
	CreatedAt        string          `json:"created_at"`
	AlreadyAssigned  bool            `json:"already_assigned"`
	Tags             []TagSummaryDTO `json:"tags,omitempty"`
}

type SubjectDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PaginationDTO = pagination.PaginationMeta

type AvailableQuestionsResponse = pagination.PaginatedList[AvailableQuestionResponse]

type ExamSetDTO struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Title           string `json:"title"`
	TotalQuestions  int    `json:"total_questions"`
	DurationMinutes int    `json:"duration_minutes"`
	PassingScore    int    `json:"passing_score"`
}

type AssignedQuestionResponse struct {
	QuestionID   string      `json:"question_id"`
	QuestionNo   int         `json:"question_no"`
	Score        float64     `json:"score"`
	QuestionText string      `json:"question_text"`
	Subject      *SubjectDTO `json:"subject,omitempty"`
	Difficulty   string      `json:"difficulty"`
	Status       string      `json:"status"`
}

type ListAssignedResponse struct {
	ExamSet              ExamSetDTO                              `json:"exam_set"`
	Items                []AssignedQuestionResponse              `json:"items"`
	Pagination           pagination.PaginationMeta               `json:"pagination"`
	IsLockedByAttempts   bool                                    `json:"is_locked_by_attempts"`
}

type BulkAddInput struct {
	QuestionIDs  []string `json:"question_ids"`
	Score        float64  `json:"score"`
	AppendToEnd  bool     `json:"append_to_end"`
}

type BulkAddResponse struct {
	ExamSetID        string `json:"exam_set_id"`
	AddedCount       int    `json:"added_count"`
	SkippedCount     int    `json:"skipped_count"`
	TotalQuestions   int    `json:"total_questions"`
	AddedQuestions   []struct {
		QuestionID string `json:"question_id"`
		QuestionNo int    `json:"question_no"`
	} `json:"added_questions"`
	SkippedQuestions []struct {
		QuestionID string `json:"question_id"`
		Reason     string `json:"reason"`
	} `json:"skipped_questions"`
}

type ReorderInput struct {
	Items []struct {
		QuestionID string `json:"question_id"`
		QuestionNo int    `json:"question_no"`
	} `json:"items"`
}

type RemoveResponse struct {
	Removed        bool `json:"removed"`
	TotalQuestions int  `json:"total_questions"`
}

type ClearAllInput struct {
	Confirm bool `json:"confirm"`
}

type ClearAllResponse struct {
	Cleared        bool `json:"cleared"`
	TotalQuestions int  `json:"total_questions"`
}

func (uc *UseCase) ListAvailable(ctx context.Context, examSetID uuid.UUID, input AvailableFilterInput) (*AvailableQuestionsResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	_ = set

	filter := esqdomain.AvailableFilter{
		Query:           input.Query,
		Difficulty:      input.Difficulty,
		Status:          input.Status,
		ExcludeAssigned: input.ExcludeAssigned,
		Page:            input.Page,
		Limit:           input.Limit,
		Sort:            input.Sort,
		Order:           input.Order,
	}
	if input.SubjectID != "" {
		sid, err := uuid.Parse(input.SubjectID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.SubjectID = sid
	}
	if input.TagID != "" {
		tid, err := uuid.Parse(input.TagID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.TagID = tid
	}

	items, total, err := uc.repo.ListAvailable(ctx, examSetID, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]AvailableQuestionResponse, len(items))
	for i, item := range items {
		resp[i] = toAvailableResponse(item)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func (uc *UseCase) ListAssigned(ctx context.Context, examSetID uuid.UUID, input AssignedFilterInput) (*ListAssignedResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}

	filter := esqdomain.AssignedFilter{
		Query: input.Query,
		Page:  input.Page,
		Limit: input.Limit,
		Sort:  input.Sort,
		Order: input.Order,
	}
	if input.SubjectID != "" {
		sid, err := uuid.Parse(input.SubjectID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.SubjectID = sid
	}
	if input.TagID != "" {
		tid, err := uuid.Parse(input.TagID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.TagID = tid
	}

	items, total, err := uc.repo.ListAssigned(ctx, examSetID, filter)
	if err != nil {
		return nil, err
	}
	locked, err := uc.repo.HasSubmittedAttempts(ctx, examSetID)
	if err != nil {
		return nil, err
	}

	resp := make([]AssignedQuestionResponse, len(items))
	for i, item := range items {
		resp[i] = toAssignedResponse(item)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	return &ListAssignedResponse{
		ExamSet: ExamSetDTO{
			ID:              set.ID.String(),
			Code:            set.Code,
			Title:           set.Title,
			TotalQuestions:  set.TotalQuestions,
			DurationMinutes: set.DurationMinutes,
			PassingScore:    set.PassingScore,
		},
		Items:              resp,
		Pagination:         pagination.NewPaginationMeta(page, limit, total),
		IsLockedByAttempts: locked,
	}, nil
}

func (uc *UseCase) BulkAdd(ctx context.Context, examSetID uuid.UUID, input BulkAddInput) (*BulkAddResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	if len(input.QuestionIDs) == 0 {
		return nil, apperrors.ErrInvalidInput
	}

	questionIDs := make([]uuid.UUID, 0, len(input.QuestionIDs))
	for _, idStr := range input.QuestionIDs {
		qID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		q, err := uc.questions.FindByID(ctx, qID)
		if err != nil {
			return nil, err
		}
		if q == nil {
			return nil, apperrors.ErrQuestionNotFound
		}
		if q.Status != qdomain.StatusPublished || !q.IsActive {
			return nil, apperrors.ErrQuestionNotPublished
		}
		questionIDs = append(questionIDs, qID)
	}

	score := input.Score
	if score <= 0 {
		score = 1
	}

	result, err := uc.repo.BulkAdd(ctx, examSetID, questionIDs, score)
	if err != nil {
		return nil, err
	}
	if err := uc.syncExamSetQuestionCount(ctx, set); err != nil {
		return nil, err
	}

	return toBulkAddResponse(result), nil
}

func (uc *UseCase) Reorder(ctx context.Context, examSetID uuid.UUID, input ReorderInput) error {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return err
	}
	if len(input.Items) == 0 {
		return apperrors.ErrInvalidInput
	}

	assigned, err := uc.repo.ListAllAssigned(ctx, examSetID)
	if err != nil {
		return err
	}
	assignedMap := make(map[uuid.UUID]bool, len(assigned))
	for _, a := range assigned {
		assignedMap[a.QuestionID] = true
	}

	seenNos := make(map[int]bool)
	items := make([]esqdomain.ReorderItem, len(input.Items))
	for i, item := range input.Items {
		qID, err := uuid.Parse(item.QuestionID)
		if err != nil {
			return apperrors.ErrInvalidUUID
		}
		if !assignedMap[qID] {
			return apperrors.ErrQuestionNotFound
		}
		if item.QuestionNo <= 0 {
			return apperrors.ErrInvalidInput
		}
		if seenNos[item.QuestionNo] {
			return apperrors.ErrInvalidInput
		}
		seenNos[item.QuestionNo] = true
		items[i] = esqdomain.ReorderItem{QuestionID: qID, QuestionNo: item.QuestionNo}
	}
	if len(seenNos) != len(assigned) {
		return apperrors.ErrInvalidInput
	}

	if err := uc.repo.Reorder(ctx, examSetID, items); err != nil {
		return err
	}
	uc.invalidateExamSetCache(ctx, set)
	return nil
}

func (uc *UseCase) Remove(ctx context.Context, examSetID, questionID uuid.UUID) (*RemoveResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	if err := uc.repo.Remove(ctx, examSetID, questionID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrQuestionNotFound
		}
		return nil, err
	}
	if err := uc.syncExamSetQuestionCount(ctx, set); err != nil {
		return nil, err
	}
	count, err := uc.repo.CountByExamSetID(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	return &RemoveResponse{Removed: true, TotalQuestions: int(count)}, nil
}

func (uc *UseCase) ClearAll(ctx context.Context, examSetID uuid.UUID, input ClearAllInput) (*ClearAllResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if !input.Confirm {
		return nil, apperrors.ErrInvalidInput
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	hasAttempts, err := uc.repo.HasAnyAttempts(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if hasAttempts {
		return nil, apperrors.ErrExamSetHasAttempts
	}
	if err := uc.repo.ClearAll(ctx, examSetID); err != nil {
		return nil, err
	}
	if err := uc.syncExamSetQuestionCount(ctx, set); err != nil {
		return nil, err
	}
	return &ClearAllResponse{Cleared: true, TotalQuestions: 0}, nil
}

func (uc *UseCase) ensureNotLocked(ctx context.Context, examSetID uuid.UUID) error {
	locked, err := uc.repo.HasSubmittedAttempts(ctx, examSetID)
	if err != nil {
		return err
	}
	if locked {
		return apperrors.ErrExamSetLockedByAttempts
	}
	return nil
}

func (uc *UseCase) requireExamSet(ctx context.Context, examSetID uuid.UUID) (*esdomain.ExamSet, error) {
	set, err := uc.sets.FindByID(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	return set, nil
}

func (uc *UseCase) syncExamSetQuestionCount(ctx context.Context, set *esdomain.ExamSet) error {
	count, err := uc.repo.CountByExamSetID(ctx, set.ID)
	if err != nil {
		return err
	}
	if err := uc.setAdmin.UpdateTotalQuestions(ctx, set.ID, int(count)); err != nil {
		return err
	}
	if err := uc.trackAdmin.RefreshCounters(ctx, set.ExamTrackID); err != nil {
		return err
	}
	uc.invalidateExamSetCache(ctx, set)
	return nil
}

func (uc *UseCase) invalidateExamSetCache(ctx context.Context, set *esdomain.ExamSet) {
	if uc.invalidator == nil || set == nil {
		return
	}
	uc.invalidator.OnExamSetChanged(ctx, set.ID.String(), set.Code)
}

func toAvailableResponse(item esqdomain.AvailableQuestion) AvailableQuestionResponse {
	resp := AvailableQuestionResponse{
		ID:               item.ID.String(),
		QuestionText:     item.QuestionText,
		Difficulty:       item.Difficulty,
		Status:           item.Status,
		CorrectChoiceKey: item.CorrectChoiceKey,
		CreatedAt:        item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		AlreadyAssigned:  item.AlreadyAssigned,
	}
	if item.Subject != nil {
		resp.Subject = &SubjectDTO{ID: item.Subject.ID, Name: item.Subject.Name}
	}
	for _, t := range item.Tags {
		resp.Tags = append(resp.Tags, TagSummaryDTO{
			ID:    t.ID,
			Name:  t.Name,
			Code:  t.Code,
			Color: t.Color,
		})
	}
	return resp
}

func toAssignedResponse(item esqdomain.AssignedQuestion) AssignedQuestionResponse {
	resp := AssignedQuestionResponse{
		QuestionID:   item.QuestionID.String(),
		QuestionNo:   item.QuestionNo,
		Score:        item.Score,
		QuestionText: item.QuestionText,
		Difficulty:   item.Difficulty,
		Status:       item.Status,
	}
	if item.Subject != nil {
		resp.Subject = &SubjectDTO{ID: item.Subject.ID, Name: item.Subject.Name}
	}
	return resp
}

func toBulkAddResponse(result esqdomain.BulkAddResult) *BulkAddResponse {
	resp := &BulkAddResponse{
		ExamSetID:        result.ExamSetID.String(),
		AddedCount:       result.AddedCount,
		SkippedCount:     result.SkippedCount,
		TotalQuestions:   result.TotalQuestions,
		AddedQuestions:   []struct {
			QuestionID string `json:"question_id"`
			QuestionNo int    `json:"question_no"`
		}{},
		SkippedQuestions: []struct {
			QuestionID string `json:"question_id"`
			Reason     string `json:"reason"`
		}{},
	}
	for _, a := range result.AddedQuestions {
		resp.AddedQuestions = append(resp.AddedQuestions, struct {
			QuestionID string `json:"question_id"`
			QuestionNo int    `json:"question_no"`
		}{QuestionID: a.QuestionID.String(), QuestionNo: a.QuestionNo})
	}
	for _, s := range result.SkippedQuestions {
		resp.SkippedQuestions = append(resp.SkippedQuestions, struct {
			QuestionID string `json:"question_id"`
			Reason     string `json:"reason"`
		}{QuestionID: s.QuestionID.String(), Reason: s.Reason})
	}
	return resp
}
