package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"virtual-exam-api/internal/examset/domain"
)

type LifecycleStopEvent struct {
	ExamSetID  uuid.UUID
	StoppedAt  time.Time
	ClaimToken uuid.UUID
}

type LifecycleStopEventModel struct {
	ExamSetID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	StoppedAt        time.Time `gorm:"primaryKey"`
	DeliveredAt      *time.Time
	ClaimToken       *uuid.UUID
	ClaimedAt        *time.Time
	DeliveryAttempts int       `gorm:"not null;default:0"`
	NextAttemptAt    time.Time `gorm:"not null;default:now();index:exam_set_lifecycle_stop_events_pending_idx,priority:1,where:delivered_at IS NULL"`
	LastError        *string
	CreatedAt        time.Time `gorm:"not null;default:now();index:exam_set_lifecycle_stop_events_pending_idx,priority:2,where:delivered_at IS NULL"`
}

type LifecycleClaimRequest struct {
	Token       uuid.UUID
	ExamSetID   *uuid.UUID
	Limit       int
	Now         time.Time
	LeaseBefore time.Time
}

type LifecycleStopOutbox interface {
	ClaimLifecycleStops(context.Context, LifecycleClaimRequest) ([]LifecycleStopEvent, error)
	MarkLifecycleStopClaimDelivered(context.Context, uuid.UUID, time.Time, uuid.UUID, time.Time) (bool, error)
	RetryLifecycleStop(context.Context, uuid.UUID, time.Time, uuid.UUID, time.Time, error) (bool, error)
}

func (LifecycleStopEventModel) TableName() string { return "exam_set_lifecycle_stop_events" }

type examSetLifecycleState struct {
	ID       uuid.UUID
	Status   string
	IsActive bool
}

func lockExamSetLifecycleState(tx *gorm.DB, id uuid.UUID) (examSetLifecycleState, error) {
	var state examSetLifecycleState
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&ExamSetModel{}).
		Select("id", "status", "is_active").
		Where("id = ?", id).
		Take(&state).Error
	return state, err
}

func isEligibleLifecycleState(status string, active bool) bool {
	return status == domain.StatusPublished && active
}

func insertLifecycleStopEvent(tx *gorm.DB, examSetID uuid.UUID, stoppedAt time.Time) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&LifecycleStopEventModel{
		ExamSetID: examSetID,
		StoppedAt: stoppedAt,
	}).Error
}

func (r *adminRepository) ListPendingLifecycleStops(ctx context.Context, examSetID uuid.UUID) ([]LifecycleStopEvent, error) {
	var models []LifecycleStopEventModel
	if err := r.db.WithContext(ctx).
		Where("exam_set_id = ? AND delivered_at IS NULL", examSetID).
		Order("stopped_at ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	events := make([]LifecycleStopEvent, len(models))
	for i := range models {
		events[i] = LifecycleStopEvent{ExamSetID: models[i].ExamSetID, StoppedAt: models[i].StoppedAt}
	}
	return events, nil
}

func (r *adminRepository) MarkLifecycleStopDelivered(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time) error {
	deliveredAt := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&LifecycleStopEventModel{}).
		Where("exam_set_id = ? AND stopped_at = ? AND delivered_at IS NULL", examSetID, stoppedAt).
		Update("delivered_at", deliveredAt).Error
}

func (r *adminRepository) ClaimLifecycleStops(ctx context.Context, request LifecycleClaimRequest) ([]LifecycleStopEvent, error) {
	if request.Limit <= 0 {
		request.Limit = 20
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	}
	if request.LeaseBefore.IsZero() {
		request.LeaseBefore = request.Now.Add(-time.Minute)
	}
	examSetID := ""
	if request.ExamSetID != nil {
		examSetID = request.ExamSetID.String()
	}
	var events []LifecycleStopEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
			WITH pending AS (
				SELECT exam_set_id, stopped_at
				FROM exam_set_lifecycle_stop_events
				WHERE delivered_at IS NULL
				  AND next_attempt_at <= ?
				  AND (claimed_at IS NULL OR claimed_at <= ?)
				  AND (? = '' OR exam_set_id::text = ?)
				ORDER BY next_attempt_at, created_at
				FOR UPDATE SKIP LOCKED
				LIMIT ?
			)
			UPDATE exam_set_lifecycle_stop_events outbox
			SET claim_token = ?, claimed_at = ?,
				delivery_attempts = delivery_attempts + 1
			FROM pending
			WHERE outbox.exam_set_id = pending.exam_set_id
			  AND outbox.stopped_at = pending.stopped_at
			RETURNING outbox.exam_set_id, outbox.stopped_at, outbox.claim_token
		`, request.Now, request.LeaseBefore, examSetID, examSetID, request.Limit,
			request.Token, request.Now).Scan(&events).Error
	})
	return events, err
}

func (r *adminRepository) MarkLifecycleStopClaimDelivered(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time, token uuid.UUID, deliveredAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&LifecycleStopEventModel{}).
		Where("exam_set_id = ? AND stopped_at = ? AND claim_token = ? AND delivered_at IS NULL", examSetID, stoppedAt, token).
		Updates(map[string]any{"delivered_at": deliveredAt, "claim_token": nil, "claimed_at": nil, "last_error": nil})
	return result.RowsAffected == 1, result.Error
}

func (r *adminRepository) RetryLifecycleStop(ctx context.Context, examSetID uuid.UUID, stoppedAt time.Time, token uuid.UUID, nextAttemptAt time.Time, deliveryErr error) (bool, error) {
	lastError := ""
	if deliveryErr != nil {
		lastError = deliveryErr.Error()
	}
	result := r.db.WithContext(ctx).Model(&LifecycleStopEventModel{}).
		Where("exam_set_id = ? AND stopped_at = ? AND claim_token = ? AND delivered_at IS NULL", examSetID, stoppedAt, token).
		Updates(map[string]any{"claim_token": nil, "claimed_at": nil, "next_attempt_at": nextAttemptAt, "last_error": lastError})
	return result.RowsAffected == 1, result.Error
}
