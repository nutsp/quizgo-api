package quota

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	markers     map[string]int
	lockGranted bool
	setCalls    int
}

func (s *fakeStore) GetUsed(_ context.Context, key string) (int, bool, error) {
	value, ok := s.markers[key]
	return value, ok, nil
}

func (s *fakeStore) SetUsed(_ context.Context, key string, used int, _ time.Duration) error {
	if s.markers == nil {
		s.markers = map[string]int{}
	}
	s.markers[key] = used
	s.setCalls++
	return nil
}

func (s *fakeStore) AcquireLock(_ context.Context, _ string, _ string, _ time.Duration) (bool, error) {
	return s.lockGranted, nil
}

func (s *fakeStore) ReleaseLock(_ context.Context, _ string, _ string) error { return nil }

type fakeHistory struct {
	count int
	from  time.Time
	to    time.Time
}

func (h *fakeHistory) CountSubmittedBetween(_ context.Context, _ uuid.UUID, from, to time.Time) (int, error) {
	h.from = from
	h.to = to
	return h.count, nil
}

func TestStatusUsesBangkokDayAndRehydratesRedisFromHistory(t *testing.T) {
	userID := uuid.New()
	store := &fakeStore{markers: map[string]int{}, lockGranted: true}
	history := &fakeHistory{count: 1}
	service := NewService(store, history, 1)
	now := time.Date(2026, 7, 17, 16, 30, 0, 0, time.UTC) // 23:30 in Bangkok

	status, err := service.Status(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Limit != 1 || status.Used != 1 || status.Remaining != 0 {
		t.Fatalf("status = %#v, want exhausted one-attempt allowance", status)
	}
	if got := history.from; !got.Equal(time.Date(2026, 7, 16, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("history from = %v, want Bangkok midnight in UTC", got)
	}
	if got := status.ResetAt; !got.Equal(time.Date(2026, 7, 17, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("reset at = %v, want next Bangkok midnight", got)
	}
	if store.setCalls != 1 {
		t.Fatalf("marker set calls = %d, want Redis rehydration", store.setCalls)
	}
}

func TestWithSubmissionConsumesAllowanceOnlyAfterSuccess(t *testing.T) {
	userID := uuid.New()
	store := &fakeStore{markers: map[string]int{}, lockGranted: true}
	service := NewService(store, &fakeHistory{}, 1)
	now := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)

	wantErr := errors.New("database write failed")
	err := service.WithSubmission(context.Background(), userID, now, func() error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithSubmission() error = %v, want callback error", err)
	}
	if store.setCalls != 0 {
		t.Fatalf("failed submission set %d markers, want 0", store.setCalls)
	}

	if err := service.WithSubmission(context.Background(), userID, now, func() error { return nil }); err != nil {
		t.Fatalf("successful WithSubmission() error = %v", err)
	}
	if store.setCalls != 1 {
		t.Fatalf("successful submission set %d markers, want 1", store.setCalls)
	}
}

func TestWithSubmissionRejectsUsedAllowanceWithoutCallingSubmit(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{markers: map[string]int{MarkerKey(userID, now): 1}, lockGranted: true}
	service := NewService(store, &fakeHistory{}, 1)
	called := false

	err := service.WithSubmission(context.Background(), userID, now, func() error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrLimitReached) {
		t.Fatalf("WithSubmission() error = %v, want ErrLimitReached", err)
	}
	if called {
		t.Fatal("submission callback called after allowance was exhausted")
	}
}

func TestWithSubmissionRejectsConcurrentDailyLock(t *testing.T) {
	service := NewService(&fakeStore{markers: map[string]int{}, lockGranted: false}, &fakeHistory{}, 1)
	err := service.WithSubmission(context.Background(), uuid.New(), time.Now(), func() error { return nil })
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("WithSubmission() error = %v, want ErrBusy", err)
	}
}

func TestServiceUsesConfiguredDailyLimit(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{markers: map[string]int{}, lockGranted: true}
	service := NewService(store, &fakeHistory{}, 2)

	if err := service.WithSubmission(context.Background(), userID, now, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	status, err := service.Status(context.Background(), userID, now)
	if err != nil {
		t.Fatal(err)
	}
	if status.Limit != 2 || status.Used != 1 || status.Remaining != 1 {
		t.Fatalf("status = %#v, want one of two attempts used", status)
	}
}

func TestUnlimitedStatusOmitsDailyCountersAndReset(t *testing.T) {
	payload, err := json.Marshal(Status{Unlimited: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != `{"unlimited":true}` {
		t.Fatalf("premium status JSON = %s, want only unlimited flag", payload)
	}
}

func TestFreeStatusIncludesZeroUsageCounters(t *testing.T) {
	payload, err := json.Marshal(Status{Limit: 1, Used: 0, Remaining: 1, ResetAt: time.Date(2026, 7, 17, 17, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"used":0`) || !strings.Contains(string(payload), `"limit":1`) {
		t.Fatalf("free status JSON = %s, want explicit zero usage and limit", payload)
	}
}
