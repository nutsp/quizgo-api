package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/examsetquestion/domain"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type Repository interface {
	ListAvailable(ctx context.Context, examSetID uuid.UUID, filter domain.AvailableFilter) ([]domain.AvailableQuestion, int64, error)
	ListAssigned(ctx context.Context, examSetID uuid.UUID) ([]domain.AssignedQuestion, error)
	BulkAdd(ctx context.Context, examSetID uuid.UUID, questionIDs []uuid.UUID, score float64) (domain.BulkAddResult, error)
	Remove(ctx context.Context, examSetID, questionID uuid.UUID) error
	Reorder(ctx context.Context, examSetID uuid.UUID, items []domain.ReorderItem) error
	ClearAll(ctx context.Context, examSetID uuid.UUID) error
	CountByExamSetID(ctx context.Context, examSetID uuid.UUID) (int64, error)
	HasSubmittedAttempts(ctx context.Context, examSetID uuid.UUID) (bool, error)
	HasAnyAttempts(ctx context.Context, examSetID uuid.UUID) (bool, error)
	AssignedQuestionIDs(ctx context.Context, examSetID uuid.UUID) (map[uuid.UUID]bool, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) ListAvailable(ctx context.Context, examSetID uuid.UUID, filter domain.AvailableFilter) ([]domain.AvailableQuestion, int64, error) {
	page, limit := pagination(filter.Page, filter.Limit)
	status := filter.Status
	if status == "" {
		status = qdomain.StatusPublished
	}

	q := r.db.WithContext(ctx).Model(&questionrepo.QuestionModel{}).
		Preload("Subject").
		Preload("Choices", func(db *gorm.DB) *gorm.DB { return db.Order("choice_key ASC") })

	if filter.Query != "" {
		q = q.Where("question_text ILIKE ?", "%"+filter.Query+"%")
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

	assignedIDs, err := r.assignedQuestionIDsTx(r.db.WithContext(ctx), examSetID)
	if err != nil {
		return nil, 0, err
	}

	if filter.ExcludeAssigned && len(assignedIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(assignedIDs))
		for id := range assignedIDs {
			ids = append(ids, id)
		}
		q = q.Where("id NOT IN ?", ids)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []questionrepo.QuestionModel
	if err := q.Order("updated_at DESC").Offset((page - 1) * limit).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	items := make([]domain.AvailableQuestion, len(models))
	for i, m := range models {
		items[i] = mapAvailableQuestion(m, assignedIDs[m.ID])
	}
	return items, total, nil
}

func (r *postgresRepository) ListAssigned(ctx context.Context, examSetID uuid.UUID) ([]domain.AssignedQuestion, error) {
	var models []questionrepo.ExamSetQuestionModel
	err := r.db.WithContext(ctx).
		Preload("Question.Subject").
		Where("exam_set_id = ?", examSetID).
		Order("question_no ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	items := make([]domain.AssignedQuestion, len(models))
	for i, m := range models {
		items[i] = mapAssignedQuestion(m)
	}
	return items, nil
}

func (r *postgresRepository) BulkAdd(ctx context.Context, examSetID uuid.UUID, questionIDs []uuid.UUID, score float64) (domain.BulkAddResult, error) {
	result := domain.BulkAddResult{
		ExamSetID:        examSetID,
		AddedQuestions:   []domain.AddedQuestion{},
		SkippedQuestions: []domain.SkippedQuestion{},
	}
	if score <= 0 {
		score = 1
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		assigned, err := r.assignedQuestionIDsTx(tx, examSetID)
		if err != nil {
			return err
		}
		maxNo, err := r.maxQuestionNoTx(tx, examSetID)
		if err != nil {
			return err
		}

		for _, qID := range questionIDs {
			if assigned[qID] {
				result.SkippedQuestions = append(result.SkippedQuestions, domain.SkippedQuestion{
					QuestionID: qID,
					Reason:     "already_assigned",
				})
				continue
			}
			maxNo++
			model := questionrepo.ExamSetQuestionModel{
				ID:         uuid.New(),
				ExamSetID:  examSetID,
				QuestionID: qID,
				QuestionNo: maxNo,
				Score:      score,
				CreatedAt:  time.Now().UTC(),
			}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
			assigned[qID] = true
			result.AddedQuestions = append(result.AddedQuestions, domain.AddedQuestion{
				QuestionID: qID,
				QuestionNo: maxNo,
			})
		}
		return nil
	})
	if err != nil {
		return domain.BulkAddResult{}, err
	}

	result.AddedCount = len(result.AddedQuestions)
	result.SkippedCount = len(result.SkippedQuestions)
	count, err := r.CountByExamSetID(ctx, examSetID)
	if err != nil {
		return domain.BulkAddResult{}, err
	}
	result.TotalQuestions = int(count)
	return result, nil
}

func (r *postgresRepository) Remove(ctx context.Context, examSetID, questionID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("exam_set_id = ? AND question_id = ?", examSetID, questionID).
			Delete(&questionrepo.ExamSetQuestionModel{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return r.renumberTx(tx, examSetID)
	})
}

func (r *postgresRepository) Reorder(ctx context.Context, examSetID uuid.UUID, items []domain.ReorderItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&questionrepo.ExamSetQuestionModel{}).
				Where("exam_set_id = ? AND question_id = ?", examSetID, item.QuestionID).
				Update("question_no", item.QuestionNo+10000).Error; err != nil {
				return err
			}
		}
		for _, item := range items {
			if err := tx.Model(&questionrepo.ExamSetQuestionModel{}).
				Where("exam_set_id = ? AND question_id = ?", examSetID, item.QuestionID).
				Update("question_no", item.QuestionNo).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postgresRepository) ClearAll(ctx context.Context, examSetID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("exam_set_id = ?", examSetID).
		Delete(&questionrepo.ExamSetQuestionModel{}).Error
}

func (r *postgresRepository) CountByExamSetID(ctx context.Context, examSetID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&questionrepo.ExamSetQuestionModel{}).
		Where("exam_set_id = ?", examSetID).
		Count(&count).Error
	return count, err
}

func (r *postgresRepository) HasSubmittedAttempts(ctx context.Context, examSetID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("exam_attempts").
		Where("exam_set_id = ? AND status IN ?", examSetID, []string{"submitted", "timeout"}).
		Count(&count).Error
	return count > 0, err
}

func (r *postgresRepository) HasAnyAttempts(ctx context.Context, examSetID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("exam_attempts").
		Where("exam_set_id = ?", examSetID).
		Count(&count).Error
	return count > 0, err
}

func (r *postgresRepository) AssignedQuestionIDs(ctx context.Context, examSetID uuid.UUID) (map[uuid.UUID]bool, error) {
	return r.assignedQuestionIDsTx(r.db.WithContext(ctx), examSetID)
}

func (r *postgresRepository) assignedQuestionIDsTx(tx *gorm.DB, examSetID uuid.UUID) (map[uuid.UUID]bool, error) {
	var rows []struct {
		QuestionID uuid.UUID
	}
	if err := tx.Model(&questionrepo.ExamSetQuestionModel{}).
		Select("question_id").
		Where("exam_set_id = ?", examSetID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		out[row.QuestionID] = true
	}
	return out, nil
}

func (r *postgresRepository) maxQuestionNoTx(tx *gorm.DB, examSetID uuid.UUID) (int, error) {
	var maxNo *int
	err := tx.Model(&questionrepo.ExamSetQuestionModel{}).
		Where("exam_set_id = ?", examSetID).
		Select("MAX(question_no)").
		Scan(&maxNo).Error
	if err != nil {
		return 0, err
	}
	if maxNo == nil {
		return 0, nil
	}
	return *maxNo, nil
}

func (r *postgresRepository) renumberTx(tx *gorm.DB, examSetID uuid.UUID) error {
	var models []questionrepo.ExamSetQuestionModel
	if err := tx.Where("exam_set_id = ?", examSetID).Order("question_no ASC").Find(&models).Error; err != nil {
		return err
	}
	for i, m := range models {
		no := i + 1
		if m.QuestionNo != no {
			if err := tx.Model(&questionrepo.ExamSetQuestionModel{}).
				Where("id = ?", m.ID).
				Update("question_no", no).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func mapAvailableQuestion(m questionrepo.QuestionModel, alreadyAssigned bool) domain.AvailableQuestion {
	item := domain.AvailableQuestion{
		ID:              m.ID,
		QuestionText:    m.QuestionText,
		Difficulty:      m.Difficulty,
		Status:          m.Status,
		CreatedAt:       m.CreatedAt,
		AlreadyAssigned: alreadyAssigned,
	}
	if m.Subject.ID != uuid.Nil {
		item.Subject = &domain.SubjectRef{
			ID:   m.Subject.ID.String(),
			Name: m.Subject.Name,
		}
	}
	for _, c := range m.Choices {
		if c.IsCorrect {
			item.CorrectChoiceKey = c.ChoiceKey
			break
		}
	}
	return item
}

func mapAssignedQuestion(m questionrepo.ExamSetQuestionModel) domain.AssignedQuestion {
	item := domain.AssignedQuestion{
		QuestionID: m.QuestionID,
		QuestionNo: m.QuestionNo,
		Score:      m.Score,
	}
	if m.Question.ID != uuid.Nil {
		item.QuestionText = m.Question.QuestionText
		item.Difficulty = m.Question.Difficulty
		item.Status = m.Question.Status
		if m.Question.Subject.ID != uuid.Nil {
			item.Subject = &domain.SubjectRef{
				ID:   m.Question.Subject.ID.String(),
				Name: m.Question.Subject.Name,
			}
		}
	}
	return item
}

func pagination(page, limit int) (int, int) {
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
