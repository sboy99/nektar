package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/sboy99/nektar/internal/ports/cache"
	"github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
)

// Bus is a synchronous in-memory event bus for tests and local dev.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]eventbus.Handler
	closed   bool
}

// NewBus creates an in-memory event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]eventbus.Handler),
	}
}

// Publish dispatches the event to all subscribers for its topic synchronously.
func (b *Bus) Publish(ctx context.Context, event eventbus.Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return context.Canceled
	}

	for _, h := range b.handlers[event.Name()] {
		if err := h(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe registers a handler for a topic.
func (b *Bus) Subscribe(_ context.Context, topic string, handler eventbus.Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

// Close shuts down the bus.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	return nil
}

// --- Cache ---

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// Cache is an in-memory cache implementation.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

// NewCache creates an in-memory cache.
func NewCache() *Cache {
	return &Cache{items: make(map[string]cacheEntry)}
}

func (c *Cache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, cache.ErrNotFound
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return nil, cache.ErrNotFound
	}
	return entry.value, nil
}

func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := cacheEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.items[key] = entry
	return nil
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// --- Storage ---

// Storage is an in-memory repository implementation.
type Storage struct {
	articles map[string]*domain.Article
	digests  map[string]*domain.Digest
	emails   map[string]*domain.Email
	mu       sync.RWMutex
}

// NewStorage creates in-memory storage.
func NewStorage() *Storage {
	return &Storage{
		articles: make(map[string]*domain.Article),
		digests:  make(map[string]*domain.Digest),
		emails:   make(map[string]*domain.Email),
	}
}

func (s *Storage) Articles() repository.ArticleRepository { return &articleRepo{s} }
func (s *Storage) Digests() repository.DigestRepository   { return &digestRepo{s} }
func (s *Storage) Emails() repository.EmailRepository     { return &emailRepo{s} }
func (s *Storage) Close() error                           { return nil }

type articleRepo struct{ s *Storage }

func (r *articleRepo) Save(_ context.Context, article *domain.Article) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.articles[article.ID] = article
	return nil
}

func (r *articleRepo) FindByID(_ context.Context, id string) (*domain.Article, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	a, ok := r.s.articles[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return a, nil
}

type digestRepo struct{ s *Storage }

func (r *digestRepo) Save(_ context.Context, digest *domain.Digest) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.digests[digest.ID] = digest
	return nil
}

func (r *digestRepo) FindByID(_ context.Context, id string) (*domain.Digest, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	d, ok := r.s.digests[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return d, nil
}

type emailRepo struct{ s *Storage }

func (r *emailRepo) Save(_ context.Context, email *domain.Email) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.emails[email.ID] = email
	return nil
}

func (r *emailRepo) FindByID(_ context.Context, id string) (*domain.Email, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	e, ok := r.s.emails[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return e, nil
}
