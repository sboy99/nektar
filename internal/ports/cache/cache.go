package cache

import (
	"context"
	"time"
)

// Cache abstracts key-value caching (Redis, in-memory, etc.).
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
