package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"virtual-exam-api/internal/examset/domain"
)

type LifecycleEventType string

const (
	LifecycleEventPublished LifecycleEventType = "published"
	LifecycleEventStopped   LifecycleEventType = "stopped"
)

type LifecycleEvent struct {
	ExamSetID   uuid.UUID
	ExamTrackID uuid.UUID
	EventType   LifecycleEventType
	EventAt     time.Time
	ClaimToken  uuid.UUID
}

type LifecycleEventModel struct {
	ExamSetID        uuid.UUID          `gorm:"type:uuid;primaryKey"`
	EventType        LifecycleEventType `gorm:"type:varchar(20);primaryKey"`
	EventAt          time.Time          `gorm:"primaryKey"`
	ExamTrackID      *uuid.UUID         `gorm:"type:uuid"`
	DeliveredAt      *time.Time
	ClaimToken       *uuid.UUID
	ClaimedAt        *time.Time
	DeliveryAttempts int       `gorm:"not null;default:0"`
	NextAttemptAt    time.Time `gorm:"not null;default:now();index:exam_set_lifecycle_events_pending_idx,priority:1,where:delivered_at IS NULL"`
	LastError        *string
	CreatedAt        time.Time `gorm:"not null;default:now();index:exam_set_lifecycle_events_pending_idx,priority:2,where:delivered_at IS NULL"`

	ExamTrack *ExamTrackJoin `gorm:"foreignKey:ExamTrackID;references:ID;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
}

type LifecycleClaimRequest struct {
	Token       uuid.UUID
	ExamSetID   *uuid.UUID
	Limit       int
	Now         time.Time
	LeaseBefore time.Time
}

type LifecycleOutbox interface {
	ClaimLifecycleEvents(context.Context, LifecycleClaimRequest) ([]LifecycleEvent, error)
	MarkLifecycleEventDelivered(context.Context, LifecycleEvent, time.Time) (bool, error)
	RetryLifecycleEvent(context.Context, LifecycleEvent, time.Time, error) (bool, error)
}

func (LifecycleEventModel) TableName() string { return "exam_set_lifecycle_events" }

type examSetLifecycleState struct {
	ID          uuid.UUID
	ExamTrackID uuid.UUID
	Status      string
	IsActive    bool
	PublishedAt *time.Time
}

func lockExamSetLifecycleState(tx *gorm.DB, id uuid.UUID) (examSetLifecycleState, error) {
	var state examSetLifecycleState
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&ExamSetModel{}).
		Select("id", "exam_track_id", "status", "is_active", "published_at").
		Where("id = ?", id).
		Take(&state).Error
	return state, err
}

func isEligibleLifecycleState(status string, active bool) bool {
	return status == domain.StatusPublished && active
}

func insertLifecycleEvent(tx *gorm.DB, event LifecycleEvent) error {
	model := LifecycleEventModel{
		ExamSetID: event.ExamSetID,
		EventType: event.EventType,
		EventAt:   event.EventAt,
	}
	if event.ExamTrackID != uuid.Nil {
		model.ExamTrackID = &event.ExamTrackID
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model).Error
}

func insertLifecycleTransitionEvent(tx *gorm.DB, before examSetLifecycleState, targetTrackID uuid.UUID, targetStatus string, targetActive bool, transitionAt time.Time) error {
	wasEligible := isEligibleLifecycleState(before.Status, before.IsActive)
	isEligible := isEligibleLifecycleState(targetStatus, targetActive)
	switch {
	case !wasEligible && isEligible:
		return insertLifecycleEvent(tx, LifecycleEvent{
			ExamSetID: before.ID, ExamTrackID: targetTrackID,
			EventType: LifecycleEventPublished, EventAt: transitionAt,
		})
	case wasEligible && !isEligible:
		return insertLifecycleEvent(tx, LifecycleEvent{
			ExamSetID: before.ID, EventType: LifecycleEventStopped, EventAt: transitionAt,
		})
	default:
		return nil
	}
}

func (r *adminRepository) ClaimLifecycleEvents(ctx context.Context, request LifecycleClaimRequest) ([]LifecycleEvent, error) {
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
	var events []LifecycleEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
			WITH pending AS (
				SELECT candidate.exam_set_id, candidate.event_type, candidate.event_at
				FROM exam_set_lifecycle_events candidate
				WHERE candidate.delivered_at IS NULL
				  AND candidate.next_attempt_at <= ?
				  AND (candidate.claimed_at IS NULL OR candidate.claimed_at <= ?)
				  AND (? = '' OR candidate.exam_set_id::text = ?)
				  AND NOT EXISTS (
					SELECT 1
					FROM exam_set_lifecycle_events earlier
					WHERE earlier.exam_set_id = candidate.exam_set_id
					  AND earlier.delivered_at IS NULL
					  AND (
						earlier.event_at < candidate.event_at OR
						(earlier.event_at = candidate.event_at AND earlier.event_type = 'published' AND candidate.event_type = 'stopped')
					  )
				  )
				ORDER BY candidate.event_at,
					CASE candidate.event_type WHEN 'published' THEN 0 ELSE 1 END,
					candidate.created_at
				FOR UPDATE OF candidate SKIP LOCKED
				LIMIT ?
			)
			UPDATE exam_set_lifecycle_events outbox
			SET claim_token = ?, claimed_at = ?,
				delivery_attempts = delivery_attempts + 1
			FROM pending
			WHERE outbox.exam_set_id = pending.exam_set_id
			  AND outbox.event_type = pending.event_type
			  AND outbox.event_at = pending.event_at
			RETURNING outbox.exam_set_id, COALESCE(outbox.exam_track_id, '00000000-0000-0000-0000-000000000000'::uuid) AS exam_track_id,
				outbox.event_type, outbox.event_at, outbox.claim_token
		`, request.Now, request.LeaseBefore, examSetID, examSetID, request.Limit,
			request.Token, request.Now).Scan(&events).Error
	})
	return events, err
}

func (r *adminRepository) MarkLifecycleEventDelivered(ctx context.Context, event LifecycleEvent, deliveredAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&LifecycleEventModel{}).
		Where("exam_set_id = ? AND event_type = ? AND event_at = ? AND claim_token = ? AND delivered_at IS NULL",
			event.ExamSetID, event.EventType, event.EventAt, event.ClaimToken).
		Updates(map[string]any{"delivered_at": deliveredAt, "claim_token": nil, "claimed_at": nil, "last_error": nil})
	return result.RowsAffected == 1, result.Error
}

func (r *adminRepository) RetryLifecycleEvent(ctx context.Context, event LifecycleEvent, nextAttemptAt time.Time, deliveryErr error) (bool, error) {
	lastError := ""
	if deliveryErr != nil {
		lastError = deliveryErr.Error()
	}
	result := r.db.WithContext(ctx).Model(&LifecycleEventModel{}).
		Where("exam_set_id = ? AND event_type = ? AND event_at = ? AND claim_token = ? AND delivered_at IS NULL",
			event.ExamSetID, event.EventType, event.EventAt, event.ClaimToken).
		Updates(map[string]any{"claim_token": nil, "claimed_at": nil, "next_attempt_at": nextAttemptAt, "last_error": lastError})
	return result.RowsAffected == 1, result.Error
}
