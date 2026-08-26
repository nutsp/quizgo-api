package quota

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var releaseLockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

type RedisStore struct {
	client *goredis.Client
}

func NewRedisStore(client *goredis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) GetUsed(ctx context.Context, key string) (int, bool, error) {
	if s == nil || s.client == nil {
		return 0, false, errors.New("daily quota redis is unavailable")
	}
	used, err := s.client.Get(ctx, key).Int()
	if err == goredis.Nil {
		return 0, false, nil
	}
	return used, err == nil, err
}

func (s *RedisStore) SetUsed(ctx context.Context, key string, used int, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return errors.New("daily quota redis is unavailable")
	}
	return s.client.Set(ctx, key, used, ttl).Err()
}

func (s *RedisStore) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	if s == nil || s.client == nil {
		return false, errors.New("daily quota redis is unavailable")
	}
	return s.client.SetNX(ctx, key, token, ttl).Result()
}

func (s *RedisStore) ReleaseLock(ctx context.Context, key, token string) error {
	if s == nil || s.client == nil {
		return errors.New("daily quota redis is unavailable")
	}
	return releaseLockScript.Run(ctx, s.client, []string{key}, token).Err()
}
