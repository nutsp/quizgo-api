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
	ExamSetID uuid.UUID
	StoppedAt time.Time
}

type LifecycleStopEventModel struct {
	ExamSetID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	StoppedAt   time.Time `gorm:"primaryKey"`
	DeliveredAt *time.Time
	CreatedAt   time.Time `gorm:"not null;default:now()"`
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
