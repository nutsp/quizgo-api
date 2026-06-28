package cache

import (
	"context"
	"time"
)

type CacheService interface {
	GetJSON(ctx context.Context, key string, dest any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	AddIndex(ctx context.Context, indexKey string, cacheKey string, ttl time.Duration) error
	DeleteByIndex(ctx context.Context, indexKey string) error
}

type noopCache struct{}

func Noop() CacheService {
	return noopCache{}
}

func (noopCache) GetJSON(_ context.Context, _ string, _ any) (bool, error) {
	return false, nil
}

func (noopCache) SetJSON(_ context.Context, _ string, _ any, _ time.Duration) error {
	return nil
}

func (noopCache) Delete(_ context.Context, _ ...string) error {
	return nil
}

func (noopCache) AddIndex(_ context.Context, _, _ string, _ time.Duration) error {
	return nil
}

func (noopCache) DeleteByIndex(_ context.Context, _ string) error {
	return nil
}
