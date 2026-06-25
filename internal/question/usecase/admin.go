package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
	subjectrepo "virtual-exam-api/internal/subject/repository"
)

type AdminUseCase struct {
	questions    questionrepo.QuestionAdminRepository
	setQuestions questionrepo.ExamSetQuestionAdminRepository
	subjects     subjectrepo.SubjectAdminRepository
	sets         examsetrepo.Repository
	setAdmin     examsetrepo.AdminRepository
	trackAdmin   trackrepo.AdminRepository
}

func NewAdminUseCase(
	questions questionrepo.QuestionAdminRepository,
	setQuestions questionrepo.ExamSetQuestionAdminRepository,
	subjects subjectrepo.SubjectAdminRepository,
	sets examsetrepo.Repository,
	setAdmin examsetrepo.AdminRepository,
	trackAdmin trackrepo.AdminRepository,
) *AdminUseCase {
	return &AdminUseCase{
		questions:    questions,
		setQuestions: setQuestions,
		subjects:     subjects,
		sets:         sets,
		setAdmin:     setAdmin,
		trackAdmin:   trackAdmin,
	}
}

type ChoiceInput struct {
	ChoiceKey   string `json:"choice_key"`
	ChoiceLabel string `json:"choice_label"`
	ChoiceText  string `json:"choice_text"`
	IsCorrect   bool   `json:"is_correct"`
}

type QuestionInput struct {
	SubjectID    string        `json:"subject_id"`
	QuestionText string        `json:"question_text"`
	Difficulty   string        `json:"difficulty"`
	Explanation  string        `json:"explanation"`
	Status       string        `json:"status"`
	Choices      []ChoiceInput `json:"choices"`
}

type ChoiceResponse struct {
	ID          string `json:"id,omitempty"`
	ChoiceKey   string `json:"choice_key"`
	ChoiceLabel string `json:"choice_label"`
	ChoiceText  string `json:"choice_text"`
	IsCorrect   bool   `json:"is_correct"`
}

type QuestionResponse struct {
	ID              string           `json:"id"`
	SubjectID       string           `json:"subject_id"`
	SubjectName     string           `json:"subject_name,omitempty"`
	QuestionText    string           `json:"question_text"`
	QuestionPreview string           `json:"question_preview,omitempty"`
	Difficulty      string           `json:"difficulty"`
	Explanation     string           `json:"explanation,omitempty"`
	Status          string           `json:"status"`
	IsActive        bool             `json:"is_active"`
	CorrectAnswer   string           `json:"correct_answer,omitempty"`
	Choices         []ChoiceResponse `json:"choices,omitempty"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

type QuestionListResponse struct {
	Items      []QuestionResponse `json:"items"`
	TotalItems int64              `json:"total_items"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
}

type ExamSetQuestionResponse struct {
	QuestionNo      int    `json:"question_no"`
	QuestionID      string `json:"question_id"`
	QuestionPreview string `json:"question_preview"`
	SubjectName     string `json:"subject_name,omitempty"`
	Difficulty      string `json:"difficulty,omitempty"`
	Score           float64 `json:"score"`
	CorrectAnswer   string `json:"correct_answer,omitempty"`
}

type AddSetQuestionInput struct {
	QuestionID string  `json:"question_id"`
	QuestionNo *int    `json:"question_no"`
	Score      float64 `json:"score"`
}

type ReorderInput struct {
	Items []struct {
		QuestionID string `json:"question_id"`
		QuestionNo int    `json:"question_no"`
	} `json:"items"`
}

func (uc *AdminUseCase) ListQuestions(ctx context.Context, filter questionrepo.QuestionAdminFilter) (*QuestionListResponse, error) {
	items, total, err := uc.questions.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	resp := make([]QuestionResponse, len(items))
	for i, q := range items {
		resp[i] = toQuestionResponse(q)
	}
	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return &QuestionListResponse{Items: resp, TotalItems: total, Page: page, Limit: limit}, nil
}

func (uc *AdminUseCase) GetQuestion(ctx context.Context, id uuid.UUID) (*QuestionResponse, error) {
	q, err := uc.questions.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, apperrors.ErrQuestionNotFound
	}
	resp := toQuestionResponse(*q)
	return &resp, nil
}

func (uc *AdminUseCase) CreateQuestion(ctx context.Context, input QuestionInput) (*QuestionResponse, error) {
	q, err := uc.buildQuestion(input)
	if err != nil {
		return nil, err
	}
	if err := uc.questions.CreateWithChoices(ctx, q); err != nil {
		return nil, err
	}
	resp := toQuestionResponse(*q)
	return &resp, nil
}

func (uc *AdminUseCase) UpdateQuestion(ctx context.Context, id uuid.UUID, input QuestionInput) (*QuestionResponse, error) {
	existing, err := uc.questions.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperrors.ErrQuestionNotFound
	}
	q, err := uc.buildQuestion(input)
	if err != nil {
		return nil, err
	}
	q.ID = id
	q.CreatedAt = existing.CreatedAt
	if err := uc.questions.UpdateWithChoices(ctx, q); err != nil {
		return nil, err
	}
	resp := toQuestionResponse(*q)
	return &resp, nil
}

func (uc *AdminUseCase) DeleteQuestion(ctx context.Context, id uuid.UUID) (archived bool, err error) {
	q, err := uc.questions.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	if q == nil {
		return false, apperrors.ErrQuestionNotFound
	}
	return uc.questions.Delete(ctx, id)
}

func (uc *AdminUseCase) ListExamSetQuestions(ctx context.Context, examSetID uuid.UUID) ([]ExamSetQuestionResponse, error) {
	set, err := uc.sets.FindByID(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	items, err := uc.setQuestions.ListByExamSetID(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	resp := make([]ExamSetQuestionResponse, len(items))
	for i, item := range items {
		resp[i] = toExamSetQuestionResponse(item)
	}
	return resp, nil
}

func (uc *AdminUseCase) AddExamSetQuestion(ctx context.Context, examSetID uuid.UUID, input AddSetQuestionInput) error {
	set, err := uc.sets.FindByID(ctx, examSetID)
	if err != nil {
		return err
	}
	if set == nil {
		return apperrors.ErrExamSetNotFound
	}
	qID, err := uuid.Parse(input.QuestionID)
	if err != nil {
		return apperrors.ErrInvalidUUID
	}
	q, err := uc.questions.FindByID(ctx, qID)
	if err != nil {
		return err
	}
	if q == nil {
		return apperrors.ErrQuestionNotFound
	}
	if q.Status != qdomain.StatusPublished || !q.IsActive {
		return apperrors.ErrQuestionNotPublished
	}
	questionNo := 0
	if input.QuestionNo != nil {
		questionNo = *input.QuestionNo
	}
	score := input.Score
	if score <= 0 {
		score = 1
	}
	if err := uc.setQuestions.AddQuestion(ctx, examSetID, qID, questionNo, score); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return apperrors.ErrDuplicateQuestion
		}
		return err
	}
	return uc.syncExamSetQuestionCount(ctx, set)
}

func (uc *AdminUseCase) RemoveExamSetQuestion(ctx context.Context, examSetID, questionID uuid.UUID) error {
	set, err := uc.sets.FindByID(ctx, examSetID)
	if err != nil {
		return err
	}
	if set == nil {
		return apperrors.ErrExamSetNotFound
	}
	if err := uc.setQuestions.RemoveQuestion(ctx, examSetID, questionID); err != nil {
		return err
	}
	return uc.syncExamSetQuestionCount(ctx, set)
}

func (uc *AdminUseCase) ReorderExamSetQuestions(ctx context.Context, examSetID uuid.UUID, input ReorderInput) error {
	set, err := uc.sets.FindByID(ctx, examSetID)
	if err != nil {
		return err
	}
	if set == nil {
		return apperrors.ErrExamSetNotFound
	}
	items := make([]questionrepo.ReorderItem, len(input.Items))
	for i, item := range input.Items {
		qID, err := uuid.Parse(item.QuestionID)
		if err != nil {
			return apperrors.ErrInvalidUUID
		}
		items[i] = questionrepo.ReorderItem{QuestionID: qID, QuestionNo: item.QuestionNo}
	}
	return uc.setQuestions.ReorderQuestions(ctx, examSetID, items)
}

func (uc *AdminUseCase) syncExamSetQuestionCount(ctx context.Context, set *domain.ExamSet) error {
	count, err := uc.setQuestions.CountByExamSetID(ctx, set.ID)
	if err != nil {
		return err
	}
	if err := uc.setAdmin.UpdateTotalQuestions(ctx, set.ID, int(count)); err != nil {
		return err
	}
	return uc.trackAdmin.RefreshCounters(ctx, set.ExamTrackID)
}

func (uc *AdminUseCase) buildQuestion(input QuestionInput) (*qdomain.Question, error) {
	if input.SubjectID == "" || input.QuestionText == "" {
		return nil, apperrors.ErrInvalidInput
	}
	subjectID, err := uuid.Parse(input.SubjectID)
	if err != nil {
		return nil, apperrors.ErrInvalidUUID
	}
	subject, err := uc.subjects.FindByID(context.Background(), subjectID)
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, apperrors.ErrNotFound
	}
	if !isValidQuestionDifficulty(input.Difficulty) || !isValidQuestionStatus(input.Status) {
		return nil, apperrors.ErrInvalidInput
	}
	choices, err := validateChoices(input.Choices)
	if err != nil {
		return nil, err
	}
	return &qdomain.Question{
		SubjectID:    subjectID,
		QuestionText: input.QuestionText,
		Explanation:  input.Explanation,
		Difficulty:   input.Difficulty,
		Status:       input.Status,
		IsActive:     input.Status != qdomain.StatusArchived,
		Subject:      &qdomain.SubjectRef{Code: subject.Code, Name: subject.Name},
		Choices:      choices,
	}, nil
}

func validateChoices(inputs []ChoiceInput) ([]qdomain.Choice, error) {
	if len(inputs) != 4 {
		return nil, apperrors.ErrInvalidChoices
	}
	correctCount := 0
	choices := make([]qdomain.Choice, 0, 4)
	seen := map[string]bool{}
	for _, in := range inputs {
		if in.ChoiceText == "" {
			return nil, apperrors.ErrInvalidChoices
		}
		if !qdomain.IsValidChoiceKey(in.ChoiceKey) {
			return nil, apperrors.ErrInvalidChoices
		}
		if seen[in.ChoiceKey] {
			return nil, apperrors.ErrInvalidChoices
		}
		seen[in.ChoiceKey] = true
		label := in.ChoiceLabel
		if label == "" {
			label = qdomain.ValidChoiceKeys[in.ChoiceKey]
		}
		if in.IsCorrect {
			correctCount++
		}
		choices = append(choices, qdomain.Choice{
			ChoiceKey:   in.ChoiceKey,
			ChoiceLabel: label,
			ChoiceText:  in.ChoiceText,
			IsCorrect:   in.IsCorrect,
		})
	}
	if correctCount != 1 {
		return nil, apperrors.ErrInvalidChoices
	}
	return choices, nil
}

func isValidQuestionDifficulty(d string) bool {
	return d == qdomain.DifficultyEasy || d == qdomain.DifficultyMedium || d == qdomain.DifficultyHard
}

func isValidQuestionStatus(s string) bool {
	return s == qdomain.StatusDraft || s == qdomain.StatusPublished || s == qdomain.StatusArchived
}

func toQuestionResponse(q qdomain.Question) QuestionResponse {
	resp := QuestionResponse{
		ID:              q.ID.String(),
		SubjectID:       q.SubjectID.String(),
		QuestionText:    q.QuestionText,
		QuestionPreview: questionrepo.TruncatePreview(q.QuestionText, 120),
		Difficulty:      q.Difficulty,
		Explanation:     q.Explanation,
		Status:          q.Status,
		IsActive:        q.IsActive,
		CreatedAt:       q.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       q.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if q.Subject != nil {
		resp.SubjectName = q.Subject.Name
	}
	for _, c := range q.Choices {
		if c.IsCorrect {
			resp.CorrectAnswer = c.ChoiceKey + " / " + c.ChoiceLabel
		}
		resp.Choices = append(resp.Choices, ChoiceResponse{
			ID:          c.ID.String(),
			ChoiceKey:   c.ChoiceKey,
			ChoiceLabel: c.ChoiceLabel,
			ChoiceText:  c.ChoiceText,
			IsCorrect:   c.IsCorrect,
		})
	}
	return resp
}

func toExamSetQuestionResponse(item qdomain.ExamSetQuestion) ExamSetQuestionResponse {
	resp := ExamSetQuestionResponse{
		QuestionNo: item.QuestionNo,
		QuestionID: item.QuestionID.String(),
		Score:      item.Score,
	}
	if item.Question != nil {
		resp.QuestionPreview = questionrepo.TruncatePreview(item.Question.QuestionText, 120)
		resp.Difficulty = item.Question.Difficulty
		if item.Question.Subject != nil {
			resp.SubjectName = item.Question.Subject.Name
		}
		for _, c := range item.Question.Choices {
			if c.IsCorrect {
				resp.CorrectAnswer = c.ChoiceKey + " / " + c.ChoiceLabel
			}
		}
	}
	return resp
}
