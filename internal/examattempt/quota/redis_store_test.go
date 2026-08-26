package quota

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisStoreMarkerAndOwnerSafeLock(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	store := NewRedisStore(client)
	ctx := context.Background()

	if used, found, err := store.GetUsed(ctx, "marker"); err != nil || found || used != 0 {
		t.Fatalf("initial GetUsed() = %v, %v, %v; want 0, false, nil", used, found, err)
	}
	if err := store.SetUsed(ctx, "marker", 2, time.Hour); err != nil {
		t.Fatal(err)
	}
	if used, found, err := store.GetUsed(ctx, "marker"); err != nil || !found || used != 2 {
		t.Fatalf("GetUsed() = %v, %v, %v; want 2, true, nil", used, found, err)
	}

	acquired, err := store.AcquireLock(ctx, "lock", "owner-a", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("AcquireLock(owner-a) = %v, %v", acquired, err)
	}
	if acquired, _ := store.AcquireLock(ctx, "lock", "owner-b", time.Minute); acquired {
		t.Fatal("second owner acquired an active lock")
	}
	if err := store.ReleaseLock(ctx, "lock", "owner-b"); err != nil {
		t.Fatal(err)
	}
	if acquired, _ := store.AcquireLock(ctx, "lock", "owner-b", time.Minute); acquired {
		t.Fatal("non-owner release removed the lock")
	}
	if err := store.ReleaseLock(ctx, "lock", "owner-a"); err != nil {
		t.Fatal(err)
	}
	if acquired, err := store.AcquireLock(ctx, "lock", "owner-b", time.Minute); err != nil || !acquired {
		t.Fatalf("AcquireLock after owner release = %v, %v", acquired, err)
	}
}
