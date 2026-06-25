package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	redisclient "virtual-exam-api/internal/redis"
)

type AttemptCacheRepository interface {
	SetAttemptState(ctx context.Context, attemptID string, ttl time.Duration) error
	SaveAnswer(ctx context.Context, attemptID string, questionNo int, choiceKey string, ttl time.Duration) error
	RemoveAnswer(ctx context.Context, attemptID string, questionNo int) error
	GetAnswers(ctx context.Context, attemptID string) (map[int]string, error)
	ClearAttempt(ctx context.Context, attemptID string) error
	SetTimer(ctx context.Context, attemptID string, expiresAt time.Time, ttl time.Duration) error
}

type redisRepository struct {
	client *goredis.Client
}

func NewRedisRepository(client *goredis.Client) AttemptCacheRepository {
	return &redisRepository{client: client}
}

func (r *redisRepository) SetAttemptState(ctx context.Context, attemptID string, ttl time.Duration) error {
	key := redisclient.AttemptStateKey(attemptID)
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"status": "in_progress",
	})
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisRepository) SaveAnswer(ctx context.Context, attemptID string, questionNo int, choiceKey string, ttl time.Duration) error {
	key := redisclient.AttemptAnswersKey(attemptID)
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, strconv.Itoa(questionNo), choiceKey)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisRepository) RemoveAnswer(ctx context.Context, attemptID string, questionNo int) error {
	key := redisclient.AttemptAnswersKey(attemptID)
	return r.client.HDel(ctx, key, strconv.Itoa(questionNo)).Err()
}

func (r *redisRepository) GetAnswers(ctx context.Context, attemptID string) (map[int]string, error) {
	key := redisclient.AttemptAnswersKey(attemptID)
	vals, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(vals))
	for k, v := range vals {
		no, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		out[no] = v
	}
	return out, nil
}

func (r *redisRepository) ClearAttempt(ctx context.Context, attemptID string) error {
	keys := []string{
		redisclient.AttemptAnswersKey(attemptID),
		redisclient.AttemptStateKey(attemptID),
		redisclient.AttemptTimerKey(attemptID),
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *redisRepository) SetTimer(ctx context.Context, attemptID string, expiresAt time.Time, ttl time.Duration) error {
	key := redisclient.AttemptTimerKey(attemptID)
	return r.client.Set(ctx, key, expiresAt.Format(time.RFC3339), ttl).Err()
}

func AttemptTTL(durationMinutes int) time.Duration {
	return time.Duration(durationMinutes+60) * time.Minute
}

func RemainingSeconds(expiresAt time.Time) int {
	remaining := int(time.Until(expiresAt).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func FormatQuestionNo(no int) string {
	return fmt.Sprintf("%d", no)
}
