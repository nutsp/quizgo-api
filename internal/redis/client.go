package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"virtual-exam-api/internal/config"
)

type Client struct {
	rdb *goredis.Client
}

type RedisClients struct {
	Content *goredis.Client
	User    *goredis.Client
	Result  *goredis.Client
	Runtime *goredis.Client
}

func NewClients(cfg *config.Config) (*RedisClients, error) {
	clients := &RedisClients{
		Content: newRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisContentDB),
		User:    newRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisUserDB),
		Result:  newRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisResultDB),
		Runtime: newRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisRuntimeDB),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := clients.Runtime.Ping(ctx).Err(); err != nil {
		_ = clients.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return clients, nil
}

func newRedisClient(addr, password string, db int) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func (c *RedisClients) Close() error {
	var firstErr error
	clients := []*goredis.Client{c.Content, c.User, c.Result, c.Runtime}
	for _, client := range clients {
		if client == nil {
			continue
		}
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// NewClient creates a single-client wrapper using the runtime DB (legacy attempt cache).
func NewClient(cfg *config.Config) (*Client, error) {
	clients, err := NewClients(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{rdb: clients.Runtime}, nil
}

func (c *Client) Raw() *goredis.Client {
	return c.rdb
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func AttemptAnswersKey(attemptID string) string {
	return fmt.Sprintf("exam_attempt:%s:answers", attemptID)
}

func AttemptStateKey(attemptID string) string {
	return fmt.Sprintf("exam_attempt:%s:state", attemptID)
}

func AttemptTimerKey(attemptID string) string {
	return fmt.Sprintf("exam_attempt:%s:timer", attemptID)
}
