package cache

import (
	"context"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	TTLDuplicateCreateLock = 30 * time.Second
	TTLSubmitLock          = 30 * time.Second
)

type RuntimeLocks struct {
	client *goredis.Client
}

func NewRuntimeLocks(client *goredis.Client) *RuntimeLocks {
	if client == nil {
		return nil
	}
	return &RuntimeLocks{client: client}
}

func (r *RuntimeLocks) TryDuplicateCreateLock(ctx context.Context, userID, examSetID string) bool {
	if r == nil || r.client == nil {
		return true
	}
	key := LockDuplicateCreateAttempt(userID, examSetID)
	ok, err := r.client.SetNX(ctx, key, "1", TTLDuplicateCreateLock).Result()
	if err != nil {
		log.Printf("runtime lock error key=%s err=%v", key, err)
		return true
	}
	return ok
}

func (r *RuntimeLocks) ReleaseDuplicateCreateLock(ctx context.Context, userID, examSetID string) {
	if r == nil || r.client == nil {
		return
	}
	key := LockDuplicateCreateAttempt(userID, examSetID)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		log.Printf("runtime unlock error key=%s err=%v", key, err)
	}
}

func (r *RuntimeLocks) TrySubmitLock(ctx context.Context, attemptID string) bool {
	if r == nil || r.client == nil {
		return true
	}
	key := LockSubmitAttempt(attemptID)
	ok, err := r.client.SetNX(ctx, key, "1", TTLSubmitLock).Result()
	if err != nil {
		log.Printf("runtime lock error key=%s err=%v", key, err)
		return true
	}
	return ok
}

func (r *RuntimeLocks) ReleaseSubmitLock(ctx context.Context, attemptID string) {
	if r == nil || r.client == nil {
		return
	}
	key := LockSubmitAttempt(attemptID)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		log.Printf("runtime unlock error key=%s err=%v", key, err)
	}
}
