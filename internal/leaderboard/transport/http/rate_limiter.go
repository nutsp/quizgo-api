package http

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	leaderboardReadsPerMinute = int64(120)
	incrementWindowScript     = `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`
)

type rateLimitStore interface {
	IncrementWindow(context.Context, string, time.Duration) (int64, error)
}

type readRateLimiter struct {
	store  rateLimitStore
	logger *log.Logger
	now    func() time.Time
}

func NewRedisReadLimiter(client *goredis.Client, logger *log.Logger) ReadLimiter {
	return newReadRateLimiter(&redisRateLimitStore{client: client}, logger, time.Now)
}

func newReadRateLimiter(store rateLimitStore, logger *log.Logger, now func() time.Time) *readRateLimiter {
	if logger == nil {
		logger = log.Default()
	}
	return &readRateLimiter{store: store, logger: logger, now: now}
}

func (l *readRateLimiter) Allow(ctx context.Context, userID uuid.UUID) bool {
	now := l.now().UTC()
	window := now.Truncate(time.Minute)
	key := fmt.Sprintf("leaderboard:reads:%s:%d", userID, window.Unix())
	ttl := window.Add(time.Minute).Sub(now)
	if ttl < time.Millisecond {
		ttl = time.Millisecond
	}

	count, err := l.store.IncrementWindow(ctx, key, ttl)
	if err != nil {
		l.logger.Printf("leaderboard read rate limit unavailable user_id=%s error=%v", userID, err)
		return true
	}
	return count <= leaderboardReadsPerMinute
}

type redisRateLimitStore struct {
	client *goredis.Client
}

func (s *redisRateLimitStore) IncrementWindow(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return s.client.Eval(
		ctx,
		incrementWindowScript,
		[]string{key},
		ttl.Milliseconds(),
	).Int64()
}
