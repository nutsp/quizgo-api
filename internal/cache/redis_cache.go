package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client  *goredis.Client
	name    string
	enabled bool
}

func NewRedisCache(client *goredis.Client, name string, enabled bool) CacheService {
	if client == nil || !enabled {
		return Noop()
	}
	return &RedisCache{client: client, name: name, enabled: enabled}
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	if c.client == nil {
		return false, nil
	}
	data, err := c.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		log.Printf("cache miss db=%s key=%s", c.name, key)
		return false, nil
	}
	if err != nil {
		log.Printf("cache get error db=%s key=%s err=%v", c.name, key, err)
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		log.Printf("cache unmarshal error db=%s key=%s err=%v", c.name, key, err)
		return false, err
	}
	log.Printf("cache hit db=%s key=%s", c.name, key)
	return true, nil
}

func (c *RedisCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("cache marshal error db=%s key=%s err=%v", c.name, key, err)
		return err
	}
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		log.Printf("cache set error db=%s key=%s err=%v", c.name, key, err)
		return err
	}
	return nil
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if c.client == nil || len(keys) == 0 {
		return nil
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		log.Printf("cache delete error db=%s keys=%v err=%v", c.name, keys, err)
		return err
	}
	return nil
}

func (c *RedisCache) AddIndex(ctx context.Context, indexKey, cacheKey string, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	pipe := c.client.Pipeline()
	pipe.SAdd(ctx, indexKey, cacheKey)
	pipe.Expire(ctx, indexKey, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("cache add index error db=%s index=%s key=%s err=%v", c.name, indexKey, cacheKey, err)
		return err
	}
	return nil
}

func (c *RedisCache) DeleteByIndex(ctx context.Context, indexKey string) error {
	if c.client == nil {
		return nil
	}
	keys, err := c.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		log.Printf("cache index members error db=%s index=%s err=%v", c.name, indexKey, err)
		return err
	}
	pipe := c.client.Pipeline()
	if len(keys) > 0 {
		pipe.Del(ctx, keys...)
	}
	pipe.Del(ctx, indexKey)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("cache delete by index error db=%s index=%s err=%v", c.name, indexKey, err)
		return err
	}
	log.Printf("cache invalidate db=%s index=%s keys=%d", c.name, indexKey, len(keys))
	return nil
}
