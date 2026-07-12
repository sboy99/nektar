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

// ReplayDLQ is a no-op for the in-memory bus (no DLQ).
func (b *Bus) ReplayDLQ(_ context.Context, _ []string, _ int) (int, error) {
	return 0, nil
}

// Trim is a no-op for the in-memory bus (no stream retention).
func (b *Bus) Trim(_ context.Context, _ []string, _ time.Duration) (int, error) {
	return 0, nil
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

func (c *Cache) PurgeExpired(_ context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	n := 0
	for key, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(c.items, key)
			n++
		}
	}
	return n, nil
}

// --- Storage ---

// Storage is an in-memory repository implementation.
type Storage struct {
	users       map[string]*domain.User
	gmailSyncs  map[string]*domain.GmailSync
	emails      map[string]*domain.Email
	articles    map[string]*domain.Article
	clusters    map[string]*domain.Cluster
	digests     map[string]*domain.Digest
	embeddings  map[string]*domain.Embedding
	llmRequests []*domain.LLMRequest
	mu          sync.RWMutex
}

// NewStorage creates in-memory storage.
func NewStorage() *Storage {
	return &Storage{
		users:       make(map[string]*domain.User),
		gmailSyncs:  make(map[string]*domain.GmailSync),
		emails:      make(map[string]*domain.Email),
		articles:    make(map[string]*domain.Article),
		clusters:    make(map[string]*domain.Cluster),
		digests:     make(map[string]*domain.Digest),
		embeddings:  make(map[string]*domain.Embedding),
		llmRequests: make([]*domain.LLMRequest, 0),
	}
}

func (s *Storage) Users() repository.UserRepository             { return &userRepo{s} }
func (s *Storage) Emails() repository.EmailRepository           { return &emailRepo{s} }
func (s *Storage) Articles() repository.ArticleRepository       { return &articleRepo{s} }
func (s *Storage) Clusters() repository.ClusterRepository       { return &clusterRepo{s} }
func (s *Storage) Digests() repository.DigestRepository         { return &digestRepo{s} }
func (s *Storage) Embeddings() repository.EmbeddingRepository   { return &embeddingRepo{s} }
func (s *Storage) LLMRequests() repository.LLMRequestRepository { return &llmRequestRepo{s} }
func (s *Storage) Close() error                                 { return nil }

type userRepo struct{ s *Storage }

func (r *userRepo) Save(_ context.Context, user *domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.users[user.ID] = user
	return nil
}

func (r *userRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	u, ok := r.s.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (r *userRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	for _, u := range r.s.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *userRepo) SaveGmailSync(_ context.Context, sync *domain.GmailSync) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.gmailSyncs[sync.UserID] = sync
	return nil
}

func (r *userRepo) GetGmailSync(_ context.Context, userID string) (*domain.GmailSync, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	sync, ok := r.s.gmailSyncs[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return sync, nil
}

func (r *userRepo) ListGmailSyncs(_ context.Context) ([]*domain.GmailSync, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	syncs := make([]*domain.GmailSync, 0, len(r.s.gmailSyncs))
	for _, sync := range r.s.gmailSyncs {
		syncs = append(syncs, sync)
	}
	return syncs, nil
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

func (r *emailRepo) FindByGmailMessageID(_ context.Context, userID, gmailMessageID string) (*domain.Email, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	for _, e := range r.s.emails {
		if e.UserID == userID && e.GmailMessageID == gmailMessageID {
			return e, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *emailRepo) ListUnprocessed(_ context.Context, userID string) ([]*domain.Email, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Email
	for _, e := range r.s.emails {
		if e.UserID == userID && e.Stage == domain.StageFetched {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *emailRepo) ListByUser(_ context.Context, userID string, limit int) ([]*domain.Email, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Email
	for _, e := range r.s.emails {
		if e.UserID == userID {
			result = append(result, e)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *emailRepo) DeleteOlderThan(_ context.Context, before time.Time) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	var deleted int64
	for id, e := range r.s.emails {
		if e.ReceivedAt.Before(before) {
			for aid, a := range r.s.articles {
				if a.EmailID == id {
					for eid, emb := range r.s.embeddings {
						if emb.ArticleID == aid {
							delete(r.s.embeddings, eid)
						}
					}
					delete(r.s.articles, aid)
				}
			}
			delete(r.s.emails, id)
			deleted++
		}
	}
	return deleted, nil
}

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

func (r *articleRepo) ListByEmail(_ context.Context, emailID string) ([]*domain.Article, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Article
	for _, a := range r.s.articles {
		if a.EmailID == emailID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *articleRepo) ListByUser(_ context.Context, userID string) ([]*domain.Article, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Article
	for _, a := range r.s.articles {
		if a.UserID == userID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *articleRepo) ListUnclustered(_ context.Context, userID string) ([]*domain.Article, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Article
	for _, a := range r.s.articles {
		if a.UserID == userID && a.Stage != domain.StageClustered {
			result = append(result, a)
		}
	}
	return result, nil
}

type clusterRepo struct{ s *Storage }

func (r *clusterRepo) Save(_ context.Context, cluster *domain.Cluster) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.clusters[cluster.ID] = cluster
	return nil
}

func (r *clusterRepo) FindByID(_ context.Context, id string) (*domain.Cluster, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	c, ok := r.s.clusters[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return c, nil
}

func (r *clusterRepo) ListByUser(_ context.Context, userID string) ([]*domain.Cluster, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Cluster
	for _, c := range r.s.clusters {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *clusterRepo) AddArticle(_ context.Context, clusterID, articleID string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.clusters[clusterID]
	if !ok {
		return repository.ErrNotFound
	}
	c.ArticleIDs = append(c.ArticleIDs, articleID)
	return nil
}

func (r *clusterRepo) UpdateCentroid(_ context.Context, clusterID string, centroid []float32) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	c, ok := r.s.clusters[clusterID]
	if !ok {
		return repository.ErrNotFound
	}
	c.Centroid = centroid
	return nil
}

func (r *clusterRepo) Delete(_ context.Context, id string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.clusters[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.s.clusters, id)
	return nil
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

func (r *digestRepo) ListUnpublished(_ context.Context, userID string) ([]*domain.Digest, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Digest
	for _, d := range r.s.digests {
		if d.UserID == userID && d.PublishStatus != domain.PublishStatusPublished {
			result = append(result, d)
		}
	}
	return result, nil
}

func (r *digestRepo) ListRecent(_ context.Context, userID string, limit int) ([]*domain.Digest, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.Digest
	for _, d := range r.s.digests {
		if d.UserID == userID {
			result = append(result, d)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

type embeddingRepo struct{ s *Storage }

func (r *embeddingRepo) Save(_ context.Context, embedding *domain.Embedding) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.embeddings[embedding.ID] = embedding
	return nil
}

func (r *embeddingRepo) FindByID(_ context.Context, id string) (*domain.Embedding, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	e, ok := r.s.embeddings[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return e, nil
}

func (r *embeddingRepo) FindByArticleID(_ context.Context, articleID string) (*domain.Embedding, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	for _, e := range r.s.embeddings {
		if e.ArticleID == articleID {
			return e, nil
		}
	}
	return nil, repository.ErrNotFound
}

type llmRequestRepo struct{ s *Storage }

func (r *llmRequestRepo) Save(_ context.Context, req *domain.LLMRequest) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.llmRequests = append(r.s.llmRequests, req)
	return nil
}

func (r *llmRequestRepo) ListByUser(_ context.Context, userID string, since time.Time) ([]*domain.LLMRequest, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	var result []*domain.LLMRequest
	for _, req := range r.s.llmRequests {
		if req.UserID == userID && !req.CreatedAt.Before(since) {
			result = append(result, req)
		}
	}
	return result, nil
}
