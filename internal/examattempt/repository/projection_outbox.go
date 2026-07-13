package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AttemptTransition string

const (
	AttemptTransitionSubmitted AttemptTransition = "submitted"
	AttemptTransitionTimedOut  AttemptTransition = "timed_out"
	AttemptTransitionUnchanged AttemptTransition = "unchanged"
)

type ProjectionOutboxModel struct {
	AttemptID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID `gorm:"type:uuid;not null"`
	ExamSetID        uuid.UUID `gorm:"type:uuid;not null"`
	ExamTrackID      uuid.UUID `gorm:"type:uuid;not null"`
	TrackCode        string    `gorm:"not null"`
	SubmittedAt      time.Time `gorm:"not null"`
	Points           float64   `gorm:"type:numeric(6,1);not null"`
	DurationSeconds  int       `gorm:"not null"`
	DeliveredAt      *time.Time
	ClaimToken       *uuid.UUID
	ClaimedAt        *time.Time
	DeliveryAttempts int       `gorm:"not null;default:0"`
	NextAttemptAt    time.Time `gorm:"not null;default:now();index:leaderboard_attempt_projection_outbox_pending_idx,priority:1,where:delivered_at IS NULL"`
	LastError        *string
	CreatedAt        time.Time `gorm:"not null;default:now();index:leaderboard_attempt_projection_outbox_pending_idx,priority:2,where:delivered_at IS NULL"`
	UpdatedAt        time.Time `gorm:"not null;default:now()"`

	Attempt   *ExamAttemptModel     `gorm:"foreignKey:AttemptID;references:ID;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
	User      *ProjectionOutboxUser `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
	ExamSet   *ExamSetJoin          `gorm:"foreignKey:ExamSetID;references:ID;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
	ExamTrack *ExamTrackJoin        `gorm:"foreignKey:ExamTrackID;references:ID;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
}

type ProjectionOutboxUser struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

func (ProjectionOutboxUser) TableName() string { return "users" }

type ProjectionOutboxEvent struct {
	AttemptID       uuid.UUID
	UserID          uuid.UUID
	ExamSetID       uuid.UUID
	ExamTrackID     uuid.UUID
	TrackCode       string
	SubmittedAt     time.Time
	Points          float64
	DurationSeconds int
	ClaimToken      uuid.UUID
}

type ProjectionClaimRequest struct {
	Token       uuid.UUID
	AttemptID   *uuid.UUID
	Limit       int
	Now         time.Time
	LeaseBefore time.Time
}

type ProjectionOutbox interface {
	ClaimProjectionEvents(context.Context, ProjectionClaimRequest) ([]ProjectionOutboxEvent, error)
	MarkProjectionDelivered(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error)
	RetryProjection(context.Context, uuid.UUID, uuid.UUID, time.Time, error) (bool, error)
}

func (ProjectionOutboxModel) TableName() string {
	return "leaderboard_attempt_projection_outbox"
}

func enqueueAttemptProjection(tx *gorm.DB, attemptID uuid.UUID) error {
	result := tx.Exec(`
		INSERT INTO leaderboard_attempt_projection_outbox (
			attempt_id, user_id, exam_set_id, exam_track_id, track_code,
			submitted_at, points, duration_seconds
		)
		SELECT
			ea.id, ea.user_id, ea.exam_set_id, ea.exam_track_id, et.code,
			ea.submitted_at, ea.score_percent, COALESCE(ea.duration_seconds, 0)
		FROM exam_attempts ea
		JOIN exam_tracks et ON et.id = ea.exam_track_id
		WHERE ea.id = ? AND ea.status IN ('submitted', 'timeout')
		ON CONFLICT (attempt_id) DO NOTHING
	`, attemptID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("enqueue attempt projection %s: inserted %d rows", attemptID, result.RowsAffected)
	}
	return nil
}

func (r *postgresRepository) ClaimProjectionEvents(ctx context.Context, request ProjectionClaimRequest) ([]ProjectionOutboxEvent, error) {
	if request.Limit <= 0 {
		request.Limit = 20
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	}
	if request.LeaseBefore.IsZero() {
		request.LeaseBefore = request.Now.Add(-time.Minute)
	}
	attemptID := ""
	if request.AttemptID != nil {
		attemptID = request.AttemptID.String()
	}
	var events []ProjectionOutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
			WITH pending AS (
				SELECT candidate.attempt_id
				FROM leaderboard_attempt_projection_outbox candidate
				JOIN exam_sets exam_set ON exam_set.id = candidate.exam_set_id
				WHERE candidate.delivered_at IS NULL
				  AND candidate.next_attempt_at <= ?
				  AND (candidate.claimed_at IS NULL OR candidate.claimed_at <= ?)
				  AND (? = '' OR candidate.attempt_id::text = ?)
				  AND NOT EXISTS (
					SELECT 1
					FROM exam_set_lifecycle_events lifecycle
					WHERE lifecycle.exam_set_id = candidate.exam_set_id
					  AND lifecycle.delivered_at IS NULL
					  AND lifecycle.event_at <= candidate.submitted_at
				  )
				ORDER BY candidate.next_attempt_at, candidate.created_at
				FOR UPDATE OF candidate, exam_set SKIP LOCKED
				LIMIT ?
			)
			UPDATE leaderboard_attempt_projection_outbox outbox
			SET claim_token = ?, claimed_at = ?,
				delivery_attempts = delivery_attempts + 1, updated_at = ?
			FROM pending
			WHERE outbox.attempt_id = pending.attempt_id
			RETURNING outbox.attempt_id, outbox.user_id, outbox.exam_set_id,
				outbox.exam_track_id, outbox.track_code, outbox.submitted_at,
				outbox.points, outbox.duration_seconds, outbox.claim_token
		`, request.Now, request.LeaseBefore, attemptID, attemptID, request.Limit,
			request.Token, request.Now, request.Now).Scan(&events).Error
	})
	return events, err
}

func (r *postgresRepository) MarkProjectionDelivered(ctx context.Context, attemptID, token uuid.UUID, deliveredAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&ProjectionOutboxModel{}).
		Where("attempt_id = ? AND claim_token = ? AND delivered_at IS NULL", attemptID, token).
		Updates(map[string]any{
			"delivered_at": deliveredAt,
			"claim_token":  nil,
			"claimed_at":   nil,
			"last_error":   nil,
			"updated_at":   deliveredAt,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *postgresRepository) RetryProjection(ctx context.Context, attemptID, token uuid.UUID, nextAttemptAt time.Time, deliveryErr error) (bool, error) {
	lastError := ""
	if deliveryErr != nil {
		lastError = deliveryErr.Error()
	}
	result := r.db.WithContext(ctx).Model(&ProjectionOutboxModel{}).
		Where("attempt_id = ? AND claim_token = ? AND delivered_at IS NULL", attemptID, token).
		Updates(map[string]any{
			"claim_token":     nil,
			"claimed_at":      nil,
			"next_attempt_at": nextAttemptAt,
			"last_error":      lastError,
			"updated_at":      time.Now().UTC(),
		})
	return result.RowsAffected == 1, result.Error
}
