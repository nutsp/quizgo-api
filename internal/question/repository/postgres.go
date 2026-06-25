package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/question/domain"
)

type SubjectModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code        string    `gorm:"uniqueIndex:uq_subjects_code;not null"`
	Name        string    `gorm:"not null"`
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SubjectModel) TableName() string { return "subjects" }

type QuestionModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubjectID    uuid.UUID `gorm:"type:uuid;not null;index"`
	QuestionText string    `gorm:"type:text;not null"`
	Explanation  string    `gorm:"type:text"`
	Difficulty   string
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Subject SubjectModel  `gorm:"foreignKey:SubjectID;references:ID"`
	Choices []ChoiceModel `gorm:"foreignKey:QuestionID;references:ID"`
}

func (QuestionModel) TableName() string { return "questions" }

type ChoiceModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	QuestionID  uuid.UUID `gorm:"type:uuid;not null;index"`
	ChoiceKey   string    `gorm:"not null"`
	ChoiceLabel string    `gorm:"not null"`
	ChoiceText  string    `gorm:"type:text;not null"`
	IsCorrect   bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ChoiceModel) TableName() string { return "choices" }

type ExamSetQuestionModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ExamSetID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_exam_set_question,priority:1;uniqueIndex:uq_exam_set_no,priority:1"`
	QuestionID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_exam_set_question,priority:2"`
	QuestionNo int       `gorm:"not null;uniqueIndex:uq_exam_set_no,priority:2"`
	Score      float64   `gorm:"type:numeric(10,2);default:1"`
	CreatedAt  time.Time

	Question QuestionModel `gorm:"foreignKey:QuestionID;references:ID"`
}

func (ExamSetQuestionModel) TableName() string { return "exam_set_questions" }

type Repository interface {
	ListByExamSetID(ctx context.Context, examSetID uuid.UUID) ([]domain.ExamSetQuestion, error)
	ListPreviewByExamSetID(ctx context.Context, examSetID uuid.UUID) ([]domain.ExamSetQuestion, error)
	GetCorrectChoicesByQuestionIDs(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID]string, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) ListByExamSetID(ctx context.Context, examSetID uuid.UUID) ([]domain.ExamSetQuestion, error) {
	var models []ExamSetQuestionModel
	err := r.db.WithContext(ctx).
		Preload("Question.Subject").
		Preload("Question.Choices", func(db *gorm.DB) *gorm.DB {
			return db.Order("choice_key ASC")
		}).
		Where("exam_set_id = ?", examSetID).
		Order("question_no ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	return mapExamSetQuestions(models), nil
}

func (r *postgresRepository) ListPreviewByExamSetID(ctx context.Context, examSetID uuid.UUID) ([]domain.ExamSetQuestion, error) {
	var models []ExamSetQuestionModel
	err := r.db.WithContext(ctx).
		Preload("Question.Subject").
		Where("exam_set_id = ?", examSetID).
		Order("question_no ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	return mapExamSetQuestions(models), nil
}

func (r *postgresRepository) GetCorrectChoicesByQuestionIDs(ctx context.Context, questionIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(questionIDs) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	var choices []ChoiceModel
	err := r.db.WithContext(ctx).
		Where("question_id IN ? AND is_correct = ?", questionIDs, true).
		Find(&choices).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]string, len(choices))
	for _, c := range choices {
		result[c.QuestionID] = c.ChoiceKey
	}
	return result, nil
}

func mapExamSetQuestions(models []ExamSetQuestionModel) []domain.ExamSetQuestion {
	out := make([]domain.ExamSetQuestion, len(models))
	for i, m := range models {
		q := domain.ExamSetQuestion{
			ID:         m.ID,
			ExamSetID:  m.ExamSetID,
			QuestionID: m.QuestionID,
			QuestionNo: m.QuestionNo,
			Score:      m.Score,
		}
		if m.Question.ID != uuid.Nil {
			question := domain.Question{
				ID:           m.Question.ID,
				SubjectID:    m.Question.SubjectID,
				QuestionText: m.Question.QuestionText,
				Explanation:  m.Question.Explanation,
				Difficulty:   m.Question.Difficulty,
			}
			if m.Question.Subject.ID != uuid.Nil {
				question.Subject = &domain.SubjectRef{
					Code: m.Question.Subject.Code,
					Name: m.Question.Subject.Name,
				}
			}
			for _, c := range m.Question.Choices {
				question.Choices = append(question.Choices, domain.Choice{
					ID:          c.ID,
					QuestionID:  c.QuestionID,
					ChoiceKey:   c.ChoiceKey,
					ChoiceLabel: c.ChoiceLabel,
					ChoiceText:  c.ChoiceText,
					IsCorrect:   c.IsCorrect,
				})
			}
			q.Question = &question
		}
		out[i] = q
	}
	return out
}
