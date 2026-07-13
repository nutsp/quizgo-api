package usecase

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	"virtual-exam-api/internal/leaderboard/domain"
)

type OutboxProjector interface {
	ProjectAttempt(context.Context, domain.ProjectionInput) (*domain.ProjectionUpdate, error)
	OnExamSetStopped(context.Context, uuid.UUID, time.Time) error
	RecordProjectionFailure(context.Context, uuid.UUID, error) error
}

type OutboxDispatcherConfig struct {
	PollInterval    time.Duration
	DeliveryTimeout time.Duration
	ClaimLease      time.Duration
	RetryBackoff    time.Duration
	BatchSize       int
	Now             func() time.Time
}

type OutboxDispatcher struct {
	attempts  attemptrepo.ProjectionOutbox
	lifecycle examsetrepo.LifecycleStopOutbox
	projector OutboxProjector
	config    OutboxDispatcherConfig
}

func NewOutboxDispatcher(attempts attemptrepo.ProjectionOutbox, lifecycle examsetrepo.LifecycleStopOutbox, projector OutboxProjector, config OutboxDispatcherConfig) *OutboxDispatcher {
	if config.PollInterval <= 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.DeliveryTimeout <= 0 {
		config.DeliveryTimeout = 5 * time.Second
	}
	if config.ClaimLease <= 0 {
		config.ClaimLease = 30 * time.Second
	}
	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 5 * time.Second
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 20
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &OutboxDispatcher{attempts: attempts, lifecycle: lifecycle, projector: projector, config: config}
}

func (d *OutboxDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := d.DrainOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("leaderboard outbox drain failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *OutboxDispatcher) DrainOnce(ctx context.Context) (int, error) {
	count := 0
	var drainErr error
	if d.attempts != nil {
		events, err := d.claimAttemptEvents(ctx, nil)
		if err != nil {
			drainErr = errors.Join(drainErr, err)
		} else {
			for i := range events {
				count++
				_, err := d.deliverAttempt(ctx, events[i])
				drainErr = errors.Join(drainErr, err)
			}
		}
	}
	if d.lifecycle != nil {
		events, err := d.claimLifecycleEvents(ctx, nil)
		if err != nil {
			drainErr = errors.Join(drainErr, err)
		} else {
			for i := range events {
				count++
				drainErr = errors.Join(drainErr, d.deliverLifecycle(ctx, events[i]))
			}
		}
	}
	return count, drainErr
}

func (d *OutboxDispatcher) DispatchAttempt(ctx context.Context, attemptID uuid.UUID) (*domain.ProjectionUpdate, error) {
	events, err := d.claimAttemptEvents(ctx, &attemptID)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return d.deliverAttempt(ctx, events[0])
}

func (d *OutboxDispatcher) RecordProjectionFailure(ctx context.Context, attemptID uuid.UUID, projectionErr error) error {
	return d.projector.RecordProjectionFailure(ctx, attemptID, projectionErr)
}

func (d *OutboxDispatcher) claimAttemptEvents(ctx context.Context, attemptID *uuid.UUID) ([]attemptrepo.ProjectionOutboxEvent, error) {
	now := d.config.Now()
	return d.attempts.ClaimProjectionEvents(ctx, attemptrepo.ProjectionClaimRequest{
		Token: uuid.New(), AttemptID: attemptID, Limit: d.config.BatchSize,
		Now: now, LeaseBefore: now.Add(-d.config.ClaimLease),
	})
}

func (d *OutboxDispatcher) claimLifecycleEvents(ctx context.Context, examSetID *uuid.UUID) ([]examsetrepo.LifecycleStopEvent, error) {
	now := d.config.Now()
	return d.lifecycle.ClaimLifecycleStops(ctx, examsetrepo.LifecycleClaimRequest{
		Token: uuid.New(), ExamSetID: examSetID, Limit: d.config.BatchSize,
		Now: now, LeaseBefore: now.Add(-d.config.ClaimLease),
	})
}

func (d *OutboxDispatcher) deliverAttempt(ctx context.Context, event attemptrepo.ProjectionOutboxEvent) (*domain.ProjectionUpdate, error) {
	deliveryCtx, cancel := context.WithTimeout(ctx, d.config.DeliveryTimeout)
	defer cancel()
	update, err := d.projector.ProjectAttempt(deliveryCtx, domain.ProjectionInput{
		AttemptID: event.AttemptID, UserID: event.UserID, ExamSetID: event.ExamSetID,
		ExamTrackID: event.ExamTrackID, TrackCode: event.TrackCode, SubmittedAt: event.SubmittedAt,
		Candidate: domain.ScoreCandidate{Points: event.Points, DurationSeconds: event.DurationSeconds, AchievedAt: event.SubmittedAt},
	})
	if err != nil {
		next := d.config.Now().Add(d.config.RetryBackoff)
		maintenanceCtx, maintenanceCancel := context.WithTimeout(context.WithoutCancel(ctx), d.config.DeliveryTimeout)
		defer maintenanceCancel()
		_, retryErr := d.attempts.RetryProjection(maintenanceCtx, event.AttemptID, event.ClaimToken, next, err)
		recordErr := d.projector.RecordProjectionFailure(maintenanceCtx, event.AttemptID, err)
		return nil, errors.Join(err, retryErr, recordErr)
	}
	maintenanceCtx, maintenanceCancel := context.WithTimeout(context.WithoutCancel(ctx), d.config.DeliveryTimeout)
	defer maintenanceCancel()
	marked, err := d.attempts.MarkProjectionDelivered(maintenanceCtx, event.AttemptID, event.ClaimToken, d.config.Now())
	if err == nil && !marked {
		err = errors.New("attempt projection claim was lost before acknowledgement")
	}
	return update, err
}

func (d *OutboxDispatcher) deliverLifecycle(ctx context.Context, event examsetrepo.LifecycleStopEvent) error {
	deliveryCtx, cancel := context.WithTimeout(ctx, d.config.DeliveryTimeout)
	defer cancel()
	err := d.projector.OnExamSetStopped(deliveryCtx, event.ExamSetID, event.StoppedAt)
	if err != nil {
		maintenanceCtx, maintenanceCancel := context.WithTimeout(context.WithoutCancel(ctx), d.config.DeliveryTimeout)
		defer maintenanceCancel()
		_, retryErr := d.lifecycle.RetryLifecycleStop(maintenanceCtx, event.ExamSetID, event.StoppedAt, event.ClaimToken, d.config.Now().Add(d.config.RetryBackoff), err)
		return errors.Join(err, retryErr)
	}
	maintenanceCtx, maintenanceCancel := context.WithTimeout(context.WithoutCancel(ctx), d.config.DeliveryTimeout)
	defer maintenanceCancel()
	marked, err := d.lifecycle.MarkLifecycleStopClaimDelivered(maintenanceCtx, event.ExamSetID, event.StoppedAt, event.ClaimToken, d.config.Now())
	if err == nil && !marked {
		err = errors.New("lifecycle stop claim was lost before acknowledgement")
	}
	return err
}
