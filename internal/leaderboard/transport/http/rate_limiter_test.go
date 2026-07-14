package http

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReadRateLimiterAllowsOneHundredTwentyAndResetsNextMinute(t *testing.T) {
	store := &rateLimitStoreFake{counts: make(map[string]int64)}
	now := time.Date(2026, time.July, 14, 10, 5, 30, 0, time.UTC)
	limiter := newReadRateLimiter(store, log.New(&bytes.Buffer{}, "", 0), func() time.Time { return now })
	userID := uuid.New()

	for request := 1; request <= 120; request++ {
		if !limiter.Allow(t.Context(), userID) {
			t.Fatalf("request %d was denied, want allowed", request)
		}
	}
	if limiter.Allow(t.Context(), userID) {
		t.Fatal("request 121 was allowed, want denied")
	}

	now = now.Add(time.Minute)
	if !limiter.Allow(t.Context(), userID) {
		t.Fatal("first request in the next minute was denied")
	}
	if len(store.counts) != 2 {
		t.Fatalf("fixed-window keys = %d, want 2", len(store.counts))
	}
}

func TestReadRateLimiterFailsOpenAndLogsStoreOutage(t *testing.T) {
	store := &rateLimitStoreFake{err: errors.New("redis unavailable")}
	var logs bytes.Buffer
	limiter := newReadRateLimiter(store, log.New(&logs, "", 0), time.Now)

	if !limiter.Allow(t.Context(), uuid.New()) {
		t.Fatal("Allow() denied during Redis outage, want fail open")
	}
	if !strings.Contains(logs.String(), "leaderboard read rate limit unavailable") ||
		!strings.Contains(logs.String(), "redis unavailable") {
		t.Fatalf("rate limit log = %q, want outage context", logs.String())
	}
}

type rateLimitStoreFake struct {
	counts map[string]int64
	err    error
}

func (f *rateLimitStoreFake) IncrementWindow(_ context.Context, key string, _ time.Duration) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.counts[key]++
	return f.counts[key], nil
}
