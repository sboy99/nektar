package redis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	portcache "github.com/sboy99/nektar/internal/ports/cache"
)

func testRedisClient(t *testing.T) *Cache {
	t.Helper()
	addr := os.Getenv("NEKTAR_REDIS_ADDR")
	if addr == "" {
		t.Skip("NEKTAR_REDIS_ADDR not set")
	}
	client := NewClient(addr, "", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}
	cache := NewCache(client)
	t.Cleanup(func() {
		_ = cache.Close()
	})
	return cache
}

func TestCacheGetMiss(t *testing.T) {
	cache := testRedisClient(t)
	ctx := context.Background()

	_, err := cache.Get(ctx, "nektar-test:missing-key")
	if !errors.Is(err, portcache.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestCacheSetGetDelete(t *testing.T) {
	cache := testRedisClient(t)
	ctx := context.Background()
	key := "nektar-test:roundtrip"

	if err := cache.Set(ctx, key, []byte("hello"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
	if err := cache.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = cache.Get(ctx, key)
	if !errors.Is(err, portcache.ErrNotFound) {
		t.Fatalf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestCacheTTLExpires(t *testing.T) {
	cache := testRedisClient(t)
	ctx := context.Background()
	key := "nektar-test:ttl"

	if err := cache.Set(ctx, key, []byte("ephemeral"), 200*time.Millisecond); err != nil {
		t.Fatalf("set: %v", err)
	}
	time.Sleep(350 * time.Millisecond)
	_, err := cache.Get(ctx, key)
	if !errors.Is(err, portcache.ErrNotFound) {
		t.Fatalf("after ttl: got %v, want ErrNotFound", err)
	}
}
