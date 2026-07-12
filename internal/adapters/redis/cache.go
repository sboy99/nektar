package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
	portcache "github.com/sboy99/nektar/internal/ports/cache"
)

// Cache implements the cache port using Redis.
type Cache struct {
	client *goredis.Client
}

// NewCache creates a Redis cache adapter.
func NewCache(client *goredis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, portcache.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// PurgeExpired is a no-op for Redis; key TTLs handle eviction.
func (c *Cache) PurgeExpired(_ context.Context) (int, error) {
	return 0, nil
}

// NewClient creates a Redis client from address and credentials.
func NewClient(addr, password string, db int) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}
