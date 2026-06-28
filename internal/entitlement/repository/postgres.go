package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/entitlement/domain"
	examsetdomain "virtual-exam-api/internal/examset/domain"
)

type EntitlementModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID          uuid.UUID  `gorm:"type:uuid;not null;index"`
	EntitlementType string     `gorm:"type:varchar(50);not null"`
	RefType         *string    `gorm:"type:varchar(50)"`
	RefID           *uuid.UUID `gorm:"type:uuid"`
	Source          string     `gorm:"type:varchar(50);not null"`
	StartsAt        time.Time  `gorm:"not null"`
	ExpiresAt       *time.Time
	IsActive        bool      `gorm:"not null;default:true;index"`
	Notes           *string   `gorm:"type:text"`
	GrantedBy       *uuid.UUID `gorm:"type:uuid"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (EntitlementModel) TableName() string { return "user_entitlements" }

type UserEntitlementSummary struct {
	UserID             uuid.UUID
	ActiveExamSetCount int
	HasPremium         bool
	PremiumExpiresAt   *time.Time
}

type Repository interface {
	Create(ctx context.Context, e *domain.Entitlement) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Entitlement, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Entitlement, int64, error)
	SummarizeActiveByUserIDs(ctx context.Context, userIDs []uuid.UUID, now time.Time) (map[uuid.UUID]UserEntitlementSummary, error)
	FindActiveExamSetEntitlement(ctx context.Context, userID, examSetID uuid.UUID, now time.Time) (*domain.Entitlement, error)
	FindActivePremiumEntitlement(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.Entitlement, error)
	HasActiveExamSetEntitlement(ctx context.Context, userID, examSetID uuid.UUID) (bool, error)
	HasActivePremiumEntitlement(ctx context.Context, userID uuid.UUID) (bool, *time.Time, error)
	FindActiveExamSetEntitlementForUpdate(ctx context.Context, userID, examSetID uuid.UUID) (*domain.Entitlement, error)
	ListAccessibleExamSets(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.AccessibleExamSetRow, error)
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, e *domain.Entitlement) error {
	m := toModel(e)
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *PostgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Entitlement, error) {
	var m EntitlementModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(&m), nil
}

func (r *PostgresRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&EntitlementModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		}).Error
}

type entitlementSummaryRow struct {
	UserID             uuid.UUID  `gorm:"column:user_id"`
	ActiveExamSetCount int        `gorm:"column:active_exam_set_count"`
	HasPremium         bool       `gorm:"column:has_premium"`
	PremiumExpiresAt   *time.Time `gorm:"column:premium_expires_at"`
}

func (r *PostgresRepository) SummarizeActiveByUserIDs(ctx context.Context, userIDs []uuid.UUID, now time.Time) (map[uuid.UUID]UserEntitlementSummary, error) {
	result := make(map[uuid.UUID]UserEntitlementSummary)
	if len(userIDs) == 0 {
		return result, nil
	}

	const query = `
SELECT
  user_id,
  COUNT(*) FILTER (
    WHERE entitlement_type = ?
      AND is_active = true
      AND starts_at <= ?
      AND (expires_at IS NULL OR expires_at > ?)
  )::int AS active_exam_set_count,
  MAX(expires_at) FILTER (
    WHERE entitlement_type = ?
      AND is_active = true
      AND starts_at <= ?
      AND (expires_at IS NULL OR expires_at > ?)
  ) AS premium_expires_at,
  COALESCE(BOOL_OR(
    entitlement_type = ?
    AND is_active = true
    AND starts_at <= ?
    AND (expires_at IS NULL OR expires_at > ?)
  ), false) AS has_premium
FROM user_entitlements
WHERE user_id IN ?
GROUP BY user_id`

	var rows []entitlementSummaryRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		domain.TypeExamSet, now, now,
		domain.TypePremium, now, now,
		domain.TypePremium, now, now,
		userIDs,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.UserID] = UserEntitlementSummary{
			UserID:             row.UserID,
			ActiveExamSetCount: row.ActiveExamSetCount,
			HasPremium:         row.HasPremium,
			PremiumExpiresAt:   row.PremiumExpiresAt,
		}
	}
	return result, nil
}

func (r *PostgresRepository) ListByUserID(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Entitlement, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	q := r.db.WithContext(ctx).Model(&EntitlementModel{}).Where("user_id = ?", userID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []EntitlementModel
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]domain.Entitlement, len(rows))
	for i, row := range rows {
		items[i] = *toDomain(&row)
	}
	return items, total, nil
}

func activeEntitlementQuery(db *gorm.DB, now time.Time) *gorm.DB {
	return db.Where("is_active = ?", true).
		Where("starts_at <= ?", now).
		Where("(expires_at IS NULL OR expires_at > ?)", now)
}

func (r *PostgresRepository) FindActiveExamSetEntitlement(ctx context.Context, userID, examSetID uuid.UUID, now time.Time) (*domain.Entitlement, error) {
	var m EntitlementModel
	refType := domain.RefTypeExamSet
	err := activeEntitlementQuery(r.db.WithContext(ctx), now).
		Where("user_id = ? AND entitlement_type = ? AND ref_type = ? AND ref_id = ?",
			userID, domain.TypeExamSet, refType, examSetID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

func (r *PostgresRepository) FindActivePremiumEntitlement(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.Entitlement, error) {
	var m EntitlementModel
	err := activeEntitlementQuery(r.db.WithContext(ctx), now).
		Where("user_id = ? AND entitlement_type = ?", userID, domain.TypePremium).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

func (r *PostgresRepository) HasActiveExamSetEntitlement(ctx context.Context, userID, examSetID uuid.UUID) (bool, error) {
	ent, err := r.FindActiveExamSetEntitlement(ctx, userID, examSetID, time.Now().UTC())
	if err != nil {
		return false, err
	}
	return ent != nil, nil
}

func (r *PostgresRepository) HasActivePremiumEntitlement(ctx context.Context, userID uuid.UUID) (bool, *time.Time, error) {
	ent, err := r.FindActivePremiumEntitlement(ctx, userID, time.Now().UTC())
	if err != nil {
		return false, nil, err
	}
	if ent == nil {
		return false, nil, nil
	}
	return true, ent.ExpiresAt, nil
}

func (r *PostgresRepository) FindActiveExamSetEntitlementForUpdate(ctx context.Context, userID, examSetID uuid.UUID) (*domain.Entitlement, error) {
	var m EntitlementModel
	refType := domain.RefTypeExamSet
	now := time.Now().UTC()
	err := activeEntitlementQuery(r.db.WithContext(ctx), now).
		Where("user_id = ? AND entitlement_type = ? AND ref_type = ? AND ref_id = ?",
			userID, domain.TypeExamSet, refType, examSetID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

type accessibleExamSetRow struct {
	ExamSetID          uuid.UUID  `gorm:"column:exam_set_id"`
	ExamTrackID        uuid.UUID  `gorm:"column:exam_track_id"`
	ExamSetCode        string     `gorm:"column:exam_set_code"`
	ExamSetTitle       string     `gorm:"column:exam_set_title"`
	ExamSetDescription string     `gorm:"column:exam_set_description"`
	CoverImageURL      *string    `gorm:"column:cover_image_url"`
	DurationMinutes    int        `gorm:"column:duration_minutes"`
	TotalQuestions     int        `gorm:"column:total_questions"`
	PassingScore       int        `gorm:"column:passing_score"`
	Difficulty         string     `gorm:"column:difficulty"`
	AccessType         string     `gorm:"column:access_type"`
	TrackCode          string     `gorm:"column:track_code"`
	TrackName          string     `gorm:"column:track_name"`
	EntitlementID      uuid.UUID  `gorm:"column:entitlement_id"`
	EntitlementSource  string     `gorm:"column:entitlement_source"`
	EntitlementStarts  time.Time  `gorm:"column:entitlement_starts_at"`
	EntitlementExpires *time.Time `gorm:"column:entitlement_expires_at"`
	AttemptID          *uuid.UUID `gorm:"column:attempt_id"`
	AttemptStatus      *string    `gorm:"column:attempt_status"`
	ScorePercent       *float64   `gorm:"column:score_percent"`
	AttemptSubmittedAt *time.Time `gorm:"column:attempt_submitted_at"`
}

func (r *PostgresRepository) ListAccessibleExamSets(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.AccessibleExamSetRow, error) {
	const query = `
SELECT
  es.id AS exam_set_id,
  es.exam_track_id,
  es.code AS exam_set_code,
  es.title AS exam_set_title,
  es.description AS exam_set_description,
  es.cover_image_url,
  es.duration_minutes,
  es.total_questions,
  es.passing_score,
  es.difficulty,
  es.access_type,
  et.code AS track_code,
  et.name AS track_name,
  ue.id AS entitlement_id,
  ue.source AS entitlement_source,
  ue.starts_at AS entitlement_starts_at,
  ue.expires_at AS entitlement_expires_at,
  la.id AS attempt_id,
  la.status AS attempt_status,
  la.score_percent,
  la.submitted_at AS attempt_submitted_at
FROM user_entitlements ue
JOIN exam_sets es ON es.id = ue.ref_id
JOIN exam_tracks et ON et.id = es.exam_track_id
LEFT JOIN LATERAL (
  SELECT ea.id, ea.status, ea.score_percent, ea.submitted_at
  FROM exam_attempts ea
  WHERE ea.user_id = ue.user_id AND ea.exam_set_id = es.id
  ORDER BY ea.created_at DESC
  LIMIT 1
) la ON true
WHERE ue.user_id = ?
  AND ue.entitlement_type = ?
  AND ue.ref_type = ?
  AND ue.is_active = true
  AND ue.starts_at <= ?
  AND (ue.expires_at IS NULL OR ue.expires_at > ?)
  AND es.status = 'published'
  AND es.is_active = true
  AND es.access_type IN ('paid', 'premium', 'private')
ORDER BY es.title ASC`

	var rows []accessibleExamSetRow
	err := r.db.WithContext(ctx).Raw(
		query,
		userID,
		domain.TypeExamSet,
		domain.RefTypeExamSet,
		now,
		now,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]domain.AccessibleExamSetRow, len(rows))
	for i, row := range rows {
		out[i] = domain.AccessibleExamSetRow{
			ExamSet: examsetdomain.ExamSet{
				ID:              row.ExamSetID,
				ExamTrackID:     row.ExamTrackID,
				Code:            row.ExamSetCode,
				Title:           row.ExamSetTitle,
				Description:     row.ExamSetDescription,
				CoverImageURL:   row.CoverImageURL,
				DurationMinutes: row.DurationMinutes,
				TotalQuestions:  row.TotalQuestions,
				PassingScore:    row.PassingScore,
				Difficulty:      row.Difficulty,
				AccessType:      row.AccessType,
				ExamTrack: &examsetdomain.ExamTrackRef{
					Code: row.TrackCode,
					Name: row.TrackName,
				},
			},
			Entitlement: domain.Entitlement{
				ID:        row.EntitlementID,
				UserID:    userID,
				Source:    row.EntitlementSource,
				StartsAt:  row.EntitlementStarts,
				ExpiresAt: row.EntitlementExpires,
				IsActive:  true,
			},
			AttemptID:          row.AttemptID,
			AttemptStatus:      row.AttemptStatus,
			ScorePercent:       row.ScorePercent,
			AttemptSubmittedAt: row.AttemptSubmittedAt,
		}
	}
	return out, nil
}

func toModel(e *domain.Entitlement) *EntitlementModel {
	return &EntitlementModel{
		ID:              e.ID,
		UserID:          e.UserID,
		EntitlementType: e.EntitlementType,
		RefType:         e.RefType,
		RefID:           e.RefID,
		Source:          e.Source,
		StartsAt:        e.StartsAt,
		ExpiresAt:       e.ExpiresAt,
		IsActive:        e.IsActive,
		Notes:           e.Notes,
		GrantedBy:       e.GrantedBy,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
}

func toDomain(m *EntitlementModel) *domain.Entitlement {
	return &domain.Entitlement{
		ID:              m.ID,
		UserID:          m.UserID,
		EntitlementType: m.EntitlementType,
		RefType:         m.RefType,
		RefID:           m.RefID,
		Source:          m.Source,
		StartsAt:        m.StartsAt,
		ExpiresAt:       m.ExpiresAt,
		IsActive:        m.IsActive,
		Notes:           m.Notes,
		GrantedBy:       m.GrantedBy,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
