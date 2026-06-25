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

func NewClient(cfg *config.Config) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
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
