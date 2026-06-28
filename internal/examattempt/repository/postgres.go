package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"virtual-exam-api/internal/examattempt/domain"
	examsetdomain "virtual-exam-api/internal/examset/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
)

type ExamAttemptModel struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID  `gorm:"type:uuid;not null;index"`
	ExamTrackID         uuid.UUID  `gorm:"type:uuid;not null;index"`
	ExamSetID           uuid.UUID  `gorm:"type:uuid;not null;index"`
	Status              string     `gorm:"not null;index"`
	StartedAt           time.Time  `gorm:"not null"`
	SubmittedAt         *time.Time
	ExpiresAt           time.Time  `gorm:"not null"`
	AccessSource        *string    `gorm:"type:varchar(50)"`
	AccessEntitlementID *uuid.UUID `gorm:"type:uuid"`
	AccessGrantedAt     *time.Time
	AccessExpiresAt     *time.Time
	DurationSeconds     *int
	Score           float64    `gorm:"type:numeric(10,2);default:0"`
	TotalScore      float64    `gorm:"type:numeric(10,2);default:0"`
	ScorePercent    float64    `gorm:"type:numeric(10,2);default:0"`
	CorrectCount    int        `gorm:"default:0"`
	WrongCount      int        `gorm:"default:0"`
	UnansweredCount int        `gorm:"default:0"`
	CreatedAt       time.Time
	UpdatedAt       time.Time

	ExamSet   ExamSetJoin   `gorm:"foreignKey:ExamSetID;references:ID"`
	ExamTrack ExamTrackJoin `gorm:"foreignKey:ExamTrackID;references:ID"`
}

type ExamSetJoin struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code            string
	Title           string
	DurationMinutes int
	TotalQuestions  int
	PassingScore    int
	AnswerSheetBlockColumns          int
	AnswerSheetQuestionsPerBlock     int
	AnswerSheetChoiceLabelStyle      string
	AnswerSheetShowHeader            bool
	AnswerSheetShowInstructions      bool
	AnswerSheetShowCandidateInfo     bool
}

func (ExamSetJoin) TableName() string { return "exam_sets" }

type ExamTrackJoin struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code string
	Name string
}

func (ExamTrackJoin) TableName() string { return "exam_tracks" }

func (ExamAttemptModel) TableName() string { return "exam_attempts" }

type ExamAnswerModel struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	AttemptID         uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_attempt_question,priority:1"`
	QuestionID        uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_attempt_question,priority:2"`
	QuestionNo        int        `gorm:"not null;index"`
	SelectedChoiceKey *string
	IsCorrect         *bool
	AnsweredAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ExamAnswerModel) TableName() string { return "exam_answers" }

type Repository interface {
	CreateAttemptWithAnswers(ctx context.Context, attempt *domain.ExamAttempt, answers []domain.ExamAnswer) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ExamAttempt, error)
	FindByIDForUser(ctx context.Context, id, userID uuid.UUID) (*domain.ExamAttempt, error)
	FindActiveAttemptByUserAndExamSet(ctx context.Context, userID, examSetID uuid.UUID) (*domain.ExamAttempt, error)
	FindLatestAttemptsByUserGroupedByExamSet(ctx context.Context, userID uuid.UUID) ([]domain.LatestAttemptSummary, error)
	FindUserActivityForExamSet(ctx context.Context, userID, examSetID uuid.UUID) (*domain.UserExamActivity, error)
	FindLatestInProgress(ctx context.Context, userID uuid.UUID) (*domain.ExamAttempt, error)
	ListAnswersByAttemptID(ctx context.Context, attemptID uuid.UUID) ([]domain.ExamAnswer, error)
	UpsertAnswer(ctx context.Context, answer *domain.ExamAnswer) error
	ClearAnswer(ctx context.Context, attemptID uuid.UUID, questionNo int) error
	UpdateAttemptSubmitted(ctx context.Context, attempt *domain.ExamAttempt, answers []domain.ExamAnswer) error
	MarkAttemptTimeout(ctx context.Context, attemptID uuid.UUID) error
	CountCompletedByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	AverageScorePercentByUser(ctx context.Context, userID uuid.UUID) (float64, error)
	ListAnswersWithQuestions(ctx context.Context, attemptID uuid.UUID) ([]AnswerWithQuestion, error)
}

type AnswerWithQuestion struct {
	Answer   domain.ExamAnswer
	Question QuestionDetail
}

type QuestionDetail struct {
	ID           uuid.UUID
	QuestionText string
	Explanation  string
	SubjectName  string
	Tags         []TagDetail
	Choices      []ChoiceDetail
}

type TagDetail struct {
	Name string
	Code string
}

type ChoiceDetail struct {
	ChoiceKey   string
	ChoiceLabel string
	ChoiceText  string
	IsCorrect   bool
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateAttemptWithAnswers(ctx context.Context, attempt *domain.ExamAttempt, answers []domain.ExamAnswer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := toAttemptModel(attempt)
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		if len(answers) > 0 {
			answerModels := make([]ExamAnswerModel, len(answers))
			for i, a := range answers {
				answerModels[i] = toAnswerModel(&a)
			}
			if err := tx.Create(&answerModels).Error; err != nil {
				return err
			}
		}
		*attempt = toAttemptDomain(&model)
		return nil
	})
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ExamAttempt, error) {
	var model ExamAttemptModel
	err := r.db.WithContext(ctx).Preload("ExamSet").Preload("ExamTrack").
		Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	attempt := toAttemptDomain(&model)
	return &attempt, nil
}

func (r *postgresRepository) FindByIDForUser(ctx context.Context, id, userID uuid.UUID) (*domain.ExamAttempt, error) {
	var model ExamAttemptModel
	err := r.db.WithContext(ctx).Preload("ExamSet").Preload("ExamTrack").
		Where("id = ? AND user_id = ?", id, userID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	attempt := toAttemptDomain(&model)
	return &attempt, nil
}

func (r *postgresRepository) FindActiveAttemptByUserAndExamSet(ctx context.Context, userID, examSetID uuid.UUID) (*domain.ExamAttempt, error) {
	var model ExamAttemptModel
	err := r.db.WithContext(ctx).Preload("ExamSet").Preload("ExamTrack").
		Where("user_id = ? AND exam_set_id = ? AND status = ?", userID, examSetID, domain.StatusInProgress).
		Order("started_at DESC").
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	attempt := toAttemptDomain(&model)
	return &attempt, nil
}

type latestAttemptRow struct {
	ExamSetID    uuid.UUID  `gorm:"column:exam_set_id"`
	AttemptID    uuid.UUID  `gorm:"column:attempt_id"`
	Status       string     `gorm:"column:status"`
	ScorePercent *float64   `gorm:"column:score_percent"`
	SubmittedAt  *time.Time `gorm:"column:submitted_at"`
	AccessSource *string    `gorm:"column:access_source"`
	StartedAt    time.Time  `gorm:"column:started_at"`
	ExpiresAt    time.Time  `gorm:"column:expires_at"`
}

func (r *postgresRepository) FindLatestAttemptsByUserGroupedByExamSet(ctx context.Context, userID uuid.UUID) ([]domain.LatestAttemptSummary, error) {
	const query = `
SELECT DISTINCT ON (ea.exam_set_id)
  ea.exam_set_id,
  ea.id AS attempt_id,
  ea.status,
  ea.score_percent,
  ea.submitted_at,
  ea.access_source,
  ea.started_at,
  ea.expires_at
FROM exam_attempts ea
JOIN exam_sets es ON es.id = ea.exam_set_id
WHERE ea.user_id = ?
  AND es.status = 'published'
  AND es.is_active = true
ORDER BY ea.exam_set_id, ea.created_at DESC`

	var rows []latestAttemptRow
	if err := r.db.WithContext(ctx).Raw(query, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]domain.LatestAttemptSummary, len(rows))
	for i, row := range rows {
		out[i] = domain.LatestAttemptSummary{
			ExamSetID:    row.ExamSetID,
			AttemptID:    row.AttemptID,
			Status:       row.Status,
			ScorePercent: row.ScorePercent,
			SubmittedAt:  row.SubmittedAt,
			AccessSource: row.AccessSource,
			StartedAt:    row.StartedAt,
			ExpiresAt:    row.ExpiresAt,
		}
	}
	return out, nil
}

func (r *postgresRepository) FindUserActivityForExamSet(ctx context.Context, userID, examSetID uuid.UUID) (*domain.UserExamActivity, error) {
	var latestSubmitted struct {
		ID           uuid.UUID
		Status       string
		ScorePercent *float64
	}
	err := r.db.WithContext(ctx).Model(&ExamAttemptModel{}).
		Select("id, status, score_percent").
		Where("user_id = ? AND exam_set_id = ? AND status IN ?", userID, examSetID, []string{domain.StatusSubmitted, domain.StatusTimeout}).
		Order("submitted_at DESC NULLS LAST, created_at DESC").
		Limit(1).
		Scan(&latestSubmitted).Error
	if err != nil {
		return nil, err
	}

	var count int64
	if err := r.db.WithContext(ctx).Model(&ExamAttemptModel{}).
		Where("user_id = ? AND exam_set_id = ? AND status IN ?", userID, examSetID, []string{domain.StatusSubmitted, domain.StatusTimeout}).
		Count(&count).Error; err != nil {
		return nil, err
	}

	if count == 0 {
		return &domain.UserExamActivity{}, nil
	}

	activity := &domain.UserExamActivity{
		HasSubmittedAttempts: true,
		LatestSubmittedAttemptID: &latestSubmitted.ID,
		LatestAttemptStatus:      &latestSubmitted.Status,
		LatestScorePercent:       latestSubmitted.ScorePercent,
	}
	return activity, nil
}

func (r *postgresRepository) MarkAttemptTimeout(ctx context.Context, attemptID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&ExamAttemptModel{}).
		Where("id = ? AND status = ?", attemptID, domain.StatusInProgress).
		Updates(map[string]any{
			"status":       domain.StatusTimeout,
			"submitted_at": now,
			"updated_at":   now,
		}).Error
}

func (r *postgresRepository) FindLatestInProgress(ctx context.Context, userID uuid.UUID) (*domain.ExamAttempt, error) {
	var model ExamAttemptModel
	err := r.db.WithContext(ctx).Preload("ExamSet").Preload("ExamTrack").
		Where("user_id = ? AND status = ?", userID, domain.StatusInProgress).
		Order("started_at DESC").
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	attempt := toAttemptDomain(&model)
	return &attempt, nil
}

func (r *postgresRepository) ListAnswersByAttemptID(ctx context.Context, attemptID uuid.UUID) ([]domain.ExamAnswer, error) {
	var models []ExamAnswerModel
	err := r.db.WithContext(ctx).
		Where("attempt_id = ?", attemptID).
		Order("question_no ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExamAnswer, len(models))
	for i := range models {
		out[i] = *toAnswerDomain(&models[i])
	}
	return out, nil
}

func (r *postgresRepository) UpsertAnswer(ctx context.Context, answer *domain.ExamAnswer) error {
	model := toAnswerModel(answer)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "attempt_id"}, {Name: "question_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"selected_choice_key", "answered_at", "updated_at",
		}),
	}).Create(&model).Error
}

func (r *postgresRepository) ClearAnswer(ctx context.Context, attemptID uuid.UUID, questionNo int) error {
	return r.db.WithContext(ctx).Model(&ExamAnswerModel{}).
		Where("attempt_id = ? AND question_no = ?", attemptID, questionNo).
		Updates(map[string]any{
			"selected_choice_key": nil,
			"answered_at":         nil,
			"updated_at":          time.Now().UTC(),
		}).Error
}

func (r *postgresRepository) UpdateAttemptSubmitted(ctx context.Context, attempt *domain.ExamAttempt, answers []domain.ExamAnswer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, a := range answers {
			if err := tx.Model(&ExamAnswerModel{}).
				Where("attempt_id = ? AND question_id = ?", a.AttemptID, a.QuestionID).
				Updates(map[string]any{
					"is_correct": a.IsCorrect,
					"updated_at": time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
		}
		model := toAttemptModel(attempt)
		return tx.Model(&ExamAttemptModel{}).Where("id = ?", attempt.ID).Updates(map[string]any{
			"status":            model.Status,
			"submitted_at":      model.SubmittedAt,
			"duration_seconds":  model.DurationSeconds,
			"score":             model.Score,
			"total_score":       model.TotalScore,
			"score_percent":     model.ScorePercent,
			"correct_count":     model.CorrectCount,
			"wrong_count":       model.WrongCount,
			"unanswered_count":  model.UnansweredCount,
			"updated_at":        time.Now().UTC(),
		}).Error
	})
}

func (r *postgresRepository) CountCompletedByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ExamAttemptModel{}).
		Where("user_id = ? AND status IN ?", userID, []string{domain.StatusSubmitted, domain.StatusTimeout}).
		Count(&count).Error
	return count, err
}

func (r *postgresRepository) AverageScorePercentByUser(ctx context.Context, userID uuid.UUID) (float64, error) {
	var avg *float64
	err := r.db.WithContext(ctx).Model(&ExamAttemptModel{}).
		Select("AVG(score_percent)").
		Where("user_id = ? AND status IN ?", userID, []string{domain.StatusSubmitted, domain.StatusTimeout}).
		Scan(&avg).Error
	if err != nil || avg == nil {
		return 0, err
	}
	return *avg, nil
}

func (r *postgresRepository) ListAnswersWithQuestions(ctx context.Context, attemptID uuid.UUID) ([]AnswerWithQuestion, error) {
	var answers []ExamAnswerModel
	if err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).Order("question_no ASC").Find(&answers).Error; err != nil {
		return nil, err
	}

	result := make([]AnswerWithQuestion, 0, len(answers))
	for _, a := range answers {
		var q questionrepo.QuestionModel
		if err := r.db.WithContext(ctx).
			Preload("Subject").
			Preload("Choices", func(db *gorm.DB) *gorm.DB { return db.Order("choice_key ASC") }).
			First(&q, "id = ?", a.QuestionID).Error; err != nil {
			return nil, err
		}
		detail := QuestionDetail{
			ID:           q.ID,
			QuestionText: q.QuestionText,
			Explanation:  q.Explanation,
		}
		if q.Subject.ID != uuid.Nil {
			detail.SubjectName = q.Subject.Name
		}
		for _, c := range q.Choices {
			detail.Choices = append(detail.Choices, ChoiceDetail{
				ChoiceKey:   c.ChoiceKey,
				ChoiceLabel: c.ChoiceLabel,
				ChoiceText:  c.ChoiceText,
				IsCorrect:   c.IsCorrect,
			})
		}
		type tagRow struct {
			Name string
			Code string
		}
		var tagRows []tagRow
		if err := r.db.WithContext(ctx).
			Table("question_tag_mappings m").
			Select("t.name, t.code").
			Joins("JOIN question_tags t ON t.id = m.tag_id").
			Where("m.question_id = ?", q.ID).
			Order("t.name ASC").
			Scan(&tagRows).Error; err != nil {
			return nil, err
		}
		for _, t := range tagRows {
			detail.Tags = append(detail.Tags, TagDetail{Name: t.Name, Code: t.Code})
		}
		result = append(result, AnswerWithQuestion{
			Answer:   *toAnswerDomain(&a),
			Question: detail,
		})
	}
	return result, nil
}

func toAttemptModel(a *domain.ExamAttempt) ExamAttemptModel {
	return ExamAttemptModel{
		ID:                  a.ID,
		UserID:              a.UserID,
		ExamTrackID:         a.ExamTrackID,
		ExamSetID:           a.ExamSetID,
		Status:              a.Status,
		StartedAt:           a.StartedAt,
		SubmittedAt:         a.SubmittedAt,
		ExpiresAt:           a.ExpiresAt,
		AccessSource:        a.AccessSource,
		AccessEntitlementID: a.AccessEntitlementID,
		AccessGrantedAt:     a.AccessGrantedAt,
		AccessExpiresAt:     a.AccessExpiresAt,
		DurationSeconds:     a.DurationSeconds,
		Score:               a.Score,
		TotalScore:          a.TotalScore,
		ScorePercent:        a.ScorePercent,
		CorrectCount:        a.CorrectCount,
		WrongCount:          a.WrongCount,
		UnansweredCount:     a.UnansweredCount,
	}
}

func toAttemptDomain(m *ExamAttemptModel) domain.ExamAttempt {
	attempt := domain.ExamAttempt{
		ID:                  m.ID,
		UserID:              m.UserID,
		ExamTrackID:         m.ExamTrackID,
		ExamSetID:           m.ExamSetID,
		Status:              m.Status,
		StartedAt:           m.StartedAt,
		SubmittedAt:         m.SubmittedAt,
		ExpiresAt:           m.ExpiresAt,
		AccessSource:        m.AccessSource,
		AccessEntitlementID: m.AccessEntitlementID,
		AccessGrantedAt:     m.AccessGrantedAt,
		AccessExpiresAt:     m.AccessExpiresAt,
		DurationSeconds:     m.DurationSeconds,
		Score:               m.Score,
		TotalScore:          m.TotalScore,
		ScorePercent:        m.ScorePercent,
		CorrectCount:        m.CorrectCount,
		WrongCount:          m.WrongCount,
		UnansweredCount:     m.UnansweredCount,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
	if m.ExamSet.Code != "" {
		attempt.ExamSet = &domain.ExamSetRef{
			Code:            m.ExamSet.Code,
			Title:           m.ExamSet.Title,
			DurationMinutes: m.ExamSet.DurationMinutes,
			TotalQuestions:  m.ExamSet.TotalQuestions,
			PassingScore:    m.ExamSet.PassingScore,
			AnswerSheetLayout: examsetdomain.LayoutFromModel(
				m.ExamSet.AnswerSheetBlockColumns,
				m.ExamSet.AnswerSheetQuestionsPerBlock,
				m.ExamSet.AnswerSheetChoiceLabelStyle,
				m.ExamSet.AnswerSheetShowHeader,
				m.ExamSet.AnswerSheetShowInstructions,
				m.ExamSet.AnswerSheetShowCandidateInfo,
			),
		}
	}
	if m.ExamTrack.Code != "" {
		attempt.ExamTrack = &domain.ExamTrackRef{
			Code: m.ExamTrack.Code,
			Name: m.ExamTrack.Name,
		}
	}
	return attempt
}

func toAnswerModel(a *domain.ExamAnswer) ExamAnswerModel {
	return ExamAnswerModel{
		ID:                a.ID,
		AttemptID:         a.AttemptID,
		QuestionID:        a.QuestionID,
		QuestionNo:        a.QuestionNo,
		SelectedChoiceKey: a.SelectedChoiceKey,
		IsCorrect:         a.IsCorrect,
		AnsweredAt:        a.AnsweredAt,
	}
}

func toAnswerDomain(m *ExamAnswerModel) *domain.ExamAnswer {
	return &domain.ExamAnswer{
		ID:                m.ID,
		AttemptID:         m.AttemptID,
		QuestionID:        m.QuestionID,
		QuestionNo:        m.QuestionNo,
		SelectedChoiceKey: m.SelectedChoiceKey,
		IsCorrect:         m.IsCorrect,
		AnsweredAt:        m.AnsweredAt,
	}
}
