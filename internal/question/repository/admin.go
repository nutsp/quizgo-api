package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/question/domain"
)

type QuestionAdminFilter struct {
	Query      string
	SubjectID  uuid.UUID
	Difficulty string
	Status     string
	Page       int
	Limit      int
}

type QuestionAdminRepository interface {
	List(ctx context.Context, filter QuestionAdminFilter) ([]domain.Question, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Question, error)
	CreateWithChoices(ctx context.Context, question *domain.Question) error
	UpdateWithChoices(ctx context.Context, question *domain.Question) error
	Delete(ctx context.Context, id uuid.UUID) (archived bool, err error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	CountAll(ctx context.Context) (int64, error)
	ListLatest(ctx context.Context, limit int) ([]domain.Question, error)
	IsUsedInAttempts(ctx context.Context, questionID uuid.UUID) (bool, error)
}

type questionAdminRepository struct {
	db *gorm.DB
}

func NewQuestionAdminRepository(db *gorm.DB) QuestionAdminRepository {
	return &questionAdminRepository{db: db}
}

func (r *questionAdminRepository) List(ctx context.Context, filter QuestionAdminFilter) ([]domain.Question, int64, error) {
	page, limit := adminPagination(filter.Page, filter.Limit)
	q := r.db.WithContext(ctx).Model(&QuestionModel{}).Preload("Subject").Preload("Choices", func(db *gorm.DB) *gorm.DB {
		return db.Order("choice_key ASC")
	})
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("question_text ILIKE ?", like)
	}
	if filter.SubjectID != uuid.Nil {
		q = q.Where("subject_id = ?", filter.SubjectID)
	}
	if filter.Difficulty != "" {
		q = q.Where("difficulty = ?", filter.Difficulty)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []QuestionModel
	err := q.Order("updated_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, 0, err
	}
	return mapQuestions(models), total, nil
}

func (r *questionAdminRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Question, error) {
	var model QuestionModel
	err := r.db.WithContext(ctx).
		Preload("Subject").
		Preload("Choices", func(db *gorm.DB) *gorm.DB { return db.Order("choice_key ASC") }).
		Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	q := mapQuestion(model)
	return &q, nil
}

func (r *questionAdminRepository) CreateWithChoices(ctx context.Context, question *domain.Question) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if question.ID == uuid.Nil {
			question.ID = uuid.New()
		}
		now := time.Now().UTC()
		question.CreatedAt = now
		question.UpdatedAt = now
		qModel := QuestionModel{
			ID:           question.ID,
			SubjectID:    question.SubjectID,
			QuestionText: question.QuestionText,
			Explanation:  question.Explanation,
			Difficulty:   question.Difficulty,
			Status:       question.Status,
			IsActive:     question.IsActive,
			CreatedAt:    question.CreatedAt,
			UpdatedAt:    question.UpdatedAt,
		}
		if err := tx.Create(&qModel).Error; err != nil {
			return err
		}
		for i := range question.Choices {
			c := &question.Choices[i]
			if c.ID == uuid.Nil {
				c.ID = uuid.New()
			}
			c.QuestionID = question.ID
			cModel := ChoiceModel{
				ID:          c.ID,
				QuestionID:  c.QuestionID,
				ChoiceKey:   c.ChoiceKey,
				ChoiceLabel: c.ChoiceLabel,
				ChoiceText:  c.ChoiceText,
				IsCorrect:   c.IsCorrect,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Create(&cModel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *questionAdminRepository) UpdateWithChoices(ctx context.Context, question *domain.Question) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		question.UpdatedAt = time.Now().UTC()
		if err := tx.Model(&QuestionModel{}).Where("id = ?", question.ID).Updates(map[string]any{
			"subject_id":    question.SubjectID,
			"question_text": question.QuestionText,
			"explanation":   question.Explanation,
			"difficulty":    question.Difficulty,
			"status":        question.Status,
			"is_active":     question.IsActive,
			"updated_at":    question.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("question_id = ?", question.ID).Delete(&ChoiceModel{}).Error; err != nil {
			return err
		}
		for i := range question.Choices {
			c := &question.Choices[i]
			if c.ID == uuid.Nil {
				c.ID = uuid.New()
			}
			c.QuestionID = question.ID
			now := time.Now().UTC()
			cModel := ChoiceModel{
				ID:          c.ID,
				QuestionID:  c.QuestionID,
				ChoiceKey:   c.ChoiceKey,
				ChoiceLabel: c.ChoiceLabel,
				ChoiceText:  c.ChoiceText,
				IsCorrect:   c.IsCorrect,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Create(&cModel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *questionAdminRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	used, err := r.IsUsedInAttempts(ctx, id)
	if err != nil {
		return false, err
	}
	if used {
		err := r.db.WithContext(ctx).Model(&QuestionModel{}).Where("id = ?", id).Updates(map[string]any{
			"status":     domain.StatusArchived,
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		}).Error
		return true, err
	}
	return false, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", id).Delete(&ChoiceModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("question_id = ?", id).Delete(&ExamSetQuestionModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&QuestionModel{}, "id = ?", id).Error
	})
}

func (r *questionAdminRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&QuestionModel{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *questionAdminRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&QuestionModel{}).Count(&count).Error
	return count, err
}

func (r *questionAdminRepository) ListLatest(ctx context.Context, limit int) ([]domain.Question, error) {
	if limit < 1 {
		limit = 5
	}
	var models []QuestionModel
	err := r.db.WithContext(ctx).Preload("Subject").Order("created_at DESC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}
	return mapQuestions(models), nil
}

func (r *questionAdminRepository) IsUsedInAttempts(ctx context.Context, questionID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("exam_answers").
		Joins("JOIN exam_set_questions ON exam_set_questions.exam_set_id = exam_answers.exam_set_id AND exam_set_questions.question_no = exam_answers.question_no").
		Where("exam_set_questions.question_id = ?", questionID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type ExamSetQuestionAdminRepository interface {
	ListByExamSetID(ctx context.Context, examSetID uuid.UUID) ([]domain.ExamSetQuestion, error)
	AddQuestion(ctx context.Context, examSetID, questionID uuid.UUID, questionNo int, score float64) error
	RemoveQuestion(ctx context.Context, examSetID, questionID uuid.UUID) error
	ReorderQuestions(ctx context.Context, examSetID uuid.UUID, items []ReorderItem) error
	CountByExamSetID(ctx context.Context, examSetID uuid.UUID) (int64, error)
	ExistsInSet(ctx context.Context, examSetID, questionID uuid.UUID) (bool, error)
	MaxQuestionNo(ctx context.Context, examSetID uuid.UUID) (int, error)
}

type ReorderItem struct {
	QuestionID uuid.UUID
	QuestionNo int
}

type examSetQuestionAdminRepository struct {
	db *gorm.DB
}

func NewExamSetQuestionAdminRepository(db *gorm.DB) ExamSetQuestionAdminRepository {
	return &examSetQuestionAdminRepository{db: db}
}

func (r *examSetQuestionAdminRepository) ListByExamSetID(ctx context.Context, examSetID uuid.UUID) ([]domain.ExamSetQuestion, error) {
	var models []ExamSetQuestionModel
	err := r.db.WithContext(ctx).
		Preload("Question.Subject").
		Preload("Question.Choices", func(db *gorm.DB) *gorm.DB { return db.Order("choice_key ASC") }).
		Where("exam_set_id = ?", examSetID).
		Order("question_no ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	return mapExamSetQuestions(models), nil
}

func (r *examSetQuestionAdminRepository) AddQuestion(ctx context.Context, examSetID, questionID uuid.UUID, questionNo int, score float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exists, err := r.existsInSetTx(tx, examSetID, questionID)
		if err != nil {
			return err
		}
		if exists {
			return gorm.ErrDuplicatedKey
		}
		if questionNo <= 0 {
			maxNo, err := r.maxQuestionNoTx(tx, examSetID)
			if err != nil {
				return err
			}
			questionNo = maxNo + 1
		}
		model := ExamSetQuestionModel{
			ID:         uuid.New(),
			ExamSetID:  examSetID,
			QuestionID: questionID,
			QuestionNo: questionNo,
			Score:      score,
			CreatedAt:  time.Now().UTC(),
		}
		if model.Score <= 0 {
			model.Score = 1
		}
		return tx.Create(&model).Error
	})
}

func (r *examSetQuestionAdminRepository) RemoveQuestion(ctx context.Context, examSetID, questionID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("exam_set_id = ? AND question_id = ?", examSetID, questionID).Delete(&ExamSetQuestionModel{}).Error; err != nil {
			return err
		}
		return r.renumberTx(tx, examSetID)
	})
}

func (r *examSetQuestionAdminRepository) ReorderQuestions(ctx context.Context, examSetID uuid.UUID, items []ReorderItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&ExamSetQuestionModel{}).
				Where("exam_set_id = ? AND question_id = ?", examSetID, item.QuestionID).
				Update("question_no", item.QuestionNo+10000).Error; err != nil {
				return err
			}
		}
		for _, item := range items {
			if err := tx.Model(&ExamSetQuestionModel{}).
				Where("exam_set_id = ? AND question_id = ?", examSetID, item.QuestionID).
				Update("question_no", item.QuestionNo).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *examSetQuestionAdminRepository) CountByExamSetID(ctx context.Context, examSetID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ExamSetQuestionModel{}).Where("exam_set_id = ?", examSetID).Count(&count).Error
	return count, err
}

func (r *examSetQuestionAdminRepository) ExistsInSet(ctx context.Context, examSetID, questionID uuid.UUID) (bool, error) {
	return r.existsInSetTx(r.db.WithContext(ctx), examSetID, questionID)
}

func (r *examSetQuestionAdminRepository) MaxQuestionNo(ctx context.Context, examSetID uuid.UUID) (int, error) {
	return r.maxQuestionNoTx(r.db.WithContext(ctx), examSetID)
}

func (r *examSetQuestionAdminRepository) existsInSetTx(tx *gorm.DB, examSetID, questionID uuid.UUID) (bool, error) {
	var count int64
	err := tx.Model(&ExamSetQuestionModel{}).Where("exam_set_id = ? AND question_id = ?", examSetID, questionID).Count(&count).Error
	return count > 0, err
}

func (r *examSetQuestionAdminRepository) maxQuestionNoTx(tx *gorm.DB, examSetID uuid.UUID) (int, error) {
	var maxNo *int
	err := tx.Model(&ExamSetQuestionModel{}).Where("exam_set_id = ?", examSetID).Select("MAX(question_no)").Scan(&maxNo).Error
	if err != nil {
		return 0, err
	}
	if maxNo == nil {
		return 0, nil
	}
	return *maxNo, nil
}

func (r *examSetQuestionAdminRepository) renumberTx(tx *gorm.DB, examSetID uuid.UUID) error {
	var models []ExamSetQuestionModel
	if err := tx.Where("exam_set_id = ?", examSetID).Order("question_no ASC").Find(&models).Error; err != nil {
		return err
	}
	for i, m := range models {
		no := i + 1
		if m.QuestionNo != no {
			if err := tx.Model(&ExamSetQuestionModel{}).Where("id = ?", m.ID).Update("question_no", no).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func mapQuestions(models []QuestionModel) []domain.Question {
	out := make([]domain.Question, len(models))
	for i, m := range models {
		out[i] = mapQuestion(m)
	}
	return out
}

func mapQuestion(m QuestionModel) domain.Question {
	q := domain.Question{
		ID:           m.ID,
		SubjectID:    m.SubjectID,
		QuestionText: m.QuestionText,
		Explanation:  m.Explanation,
		Difficulty:   m.Difficulty,
		Status:       m.Status,
		IsActive:     m.IsActive,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if m.Subject.ID != uuid.Nil {
		q.Subject = &domain.SubjectRef{Code: m.Subject.Code, Name: m.Subject.Name}
	}
	for _, c := range m.Choices {
		q.Choices = append(q.Choices, domain.Choice{
			ID:          c.ID,
			QuestionID:  c.QuestionID,
			ChoiceKey:   c.ChoiceKey,
			ChoiceLabel: c.ChoiceLabel,
			ChoiceText:  c.ChoiceText,
			IsCorrect:   c.IsCorrect,
		})
	}
	return q
}

func adminPagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func CountAllAttempts(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Table("exam_attempts").Count(&count).Error
	return count, err
}

func CountAllTracks(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Table("exam_tracks").Count(&count).Error
	return count, err
}

func CountAllExamSets(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Table("exam_sets").Count(&count).Error
	return count, err
}

func TruncatePreview(text string, max int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}
