package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	"virtual-exam-api/internal/leaderboard/domain"
)

type dispatcherAttemptOutbox struct {
	mu      sync.Mutex
	event   *attemptrepo.ProjectionOutboxEvent
	claimed bool
	retries int
}

func (f *dispatcherAttemptOutbox) ClaimProjectionEvents(_ context.Context, request attemptrepo.ProjectionClaimRequest) ([]attemptrepo.ProjectionOutboxEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.event == nil || f.claimed || (request.AttemptID != nil && *request.AttemptID != f.event.AttemptID) {
		return nil, nil
	}
	f.claimed = true
	event := *f.event
	event.ClaimToken = request.Token
	return []attemptrepo.ProjectionOutboxEvent{event}, nil
}

func (f *dispatcherAttemptOutbox) MarkProjectionDelivered(_ context.Context, attemptID, token uuid.UUID, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.event == nil || f.event.AttemptID != attemptID || !f.claimed {
		return false, nil
	}
	f.event = nil
	return true, nil
}

func (f *dispatcherAttemptOutbox) RetryProjection(_ context.Context, attemptID, token uuid.UUID, _ time.Time, _ error) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.event == nil || f.event.AttemptID != attemptID || !f.claimed {
		return false, nil
	}
	f.claimed = false
	f.retries++
	return true, nil
}

type dispatcherLifecycleOutbox struct {
}

func (dispatcherLifecycleOutbox) ClaimLifecycleStops(context.Context, examsetrepo.LifecycleClaimRequest) ([]examsetrepo.LifecycleStopEvent, error) {
	return nil, nil
}

func (dispatcherLifecycleOutbox) MarkLifecycleStopClaimDelivered(context.Context, uuid.UUID, time.Time, uuid.UUID, time.Time) (bool, error) {
	return false, nil
}

func (dispatcherLifecycleOutbox) RetryLifecycleStop(context.Context, uuid.UUID, time.Time, uuid.UUID, time.Time, error) (bool, error) {
	return false, nil
}

type dispatcherProjector struct {
	mu       sync.Mutex
	projects int
	failures int
	err      error
}

func (p *dispatcherProjector) ProjectAttempt(context.Context, domain.ProjectionInput) (*domain.ProjectionUpdate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.projects++
	return &domain.ProjectionUpdate{CurrentRank: 7}, p.err
}

func (p *dispatcherProjector) OnExamSetStopped(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (p *dispatcherProjector) RecordProjectionFailure(context.Context, uuid.UUID, error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures++
	return nil
}

func TestOutboxDispatcherImmediateAndConcurrentDrainClaimOnce(t *testing.T) {
	event := dispatcherProjectionEvent()
	outbox := &dispatcherAttemptOutbox{event: &event}
	projector := &dispatcherProjector{}
	dispatcher := NewOutboxDispatcher(outbox, dispatcherLifecycleOutbox{}, projector, OutboxDispatcherConfig{})

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			<-start
			_, _ = dispatcher.DispatchAttempt(context.Background(), event.AttemptID)
		}()
	}
	close(start)
	wg.Wait()

	if projector.projects != 1 {
		t.Fatalf("project calls = %d, want 1", projector.projects)
	}
	if outbox.event != nil {
		t.Fatal("delivered event remains pending")
	}
}

func TestOutboxDispatcherFailureRemainsPendingAndRecordsFailure(t *testing.T) {
	event := dispatcherProjectionEvent()
	outbox := &dispatcherAttemptOutbox{event: &event}
	projector := &dispatcherProjector{err: errors.New("projector unavailable")}
	dispatcher := NewOutboxDispatcher(outbox, dispatcherLifecycleOutbox{}, projector, OutboxDispatcherConfig{})

	update, err := dispatcher.DispatchAttempt(context.Background(), event.AttemptID)
	if err == nil || update != nil {
		t.Fatalf("DispatchAttempt() = %#v, %v, want nil/error", update, err)
	}
	if outbox.event == nil || outbox.claimed || outbox.retries != 1 {
		t.Fatalf("outbox after failure = event:%v claimed:%v retries:%d", outbox.event != nil, outbox.claimed, outbox.retries)
	}
	if projector.failures != 1 {
		t.Fatalf("failure records = %d, want 1", projector.failures)
	}
}

func TestOutboxDispatcherRunStopsOnContextCancellation(t *testing.T) {
	outbox := &dispatcherAttemptOutbox{}
	dispatcher := NewOutboxDispatcher(outbox, dispatcherLifecycleOutbox{}, &dispatcherProjector{}, OutboxDispatcherConfig{
		PollInterval: time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		dispatcher.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not stop after cancellation")
	}
}

func dispatcherProjectionEvent() attemptrepo.ProjectionOutboxEvent {
	return attemptrepo.ProjectionOutboxEvent{
		AttemptID:       uuid.New(),
		UserID:          uuid.New(),
		ExamSetID:       uuid.New(),
		ExamTrackID:     uuid.New(),
		TrackCode:       "police",
		SubmittedAt:     time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
		Points:          80,
		DurationSeconds: 600,
	}
}
