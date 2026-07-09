package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/internal/shared/errors"
	"github.com/sboy99/nektar/shared/domain"
)

// Storage implements repository.Storage using PostgreSQL.
type Storage struct {
	pool *pgxpool.Pool
}

// NewStorage connects to PostgreSQL and returns a storage adapter.
func NewStorage(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Storage{pool: pool}, nil
}

func (s *Storage) Users() repository.UserRepository             { return &userRepo{s} }
func (s *Storage) Emails() repository.EmailRepository           { return &emailRepo{s} }
func (s *Storage) Articles() repository.ArticleRepository       { return &articleRepo{s} }
func (s *Storage) Clusters() repository.ClusterRepository       { return &clusterRepo{s} }
func (s *Storage) Digests() repository.DigestRepository         { return &digestRepo{s} }
func (s *Storage) Embeddings() repository.EmbeddingRepository   { return &embeddingRepo{s} }
func (s *Storage) LLMRequests() repository.LLMRequestRepository { return &llmRequestRepo{s} }

func (s *Storage) Close() error {
	s.pool.Close()
	return nil
}

type userRepo struct{ s *Storage }

func (r *userRepo) Save(ctx context.Context, user *domain.User) error {
	return errors.ErrNotImplemented
}
func (r *userRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, errors.ErrNotImplemented
}
func (r *userRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, errors.ErrNotImplemented
}
func (r *userRepo) SaveGmailSync(ctx context.Context, sync *domain.GmailSync) error {
	return errors.ErrNotImplemented
}
func (r *userRepo) GetGmailSync(ctx context.Context, userID string) (*domain.GmailSync, error) {
	return nil, errors.ErrNotImplemented
}

type emailRepo struct{ s *Storage }

func (r *emailRepo) Save(ctx context.Context, email *domain.Email) error {
	return errors.ErrNotImplemented
}
func (r *emailRepo) FindByID(ctx context.Context, id string) (*domain.Email, error) {
	return nil, errors.ErrNotImplemented
}
func (r *emailRepo) FindByGmailMessageID(ctx context.Context, userID, gmailMessageID string) (*domain.Email, error) {
	return nil, errors.ErrNotImplemented
}
func (r *emailRepo) ListUnprocessed(ctx context.Context, userID string) ([]*domain.Email, error) {
	return nil, errors.ErrNotImplemented
}
func (r *emailRepo) ListByUser(ctx context.Context, userID string, limit int) ([]*domain.Email, error) {
	return nil, errors.ErrNotImplemented
}

type articleRepo struct{ s *Storage }

func (r *articleRepo) Save(ctx context.Context, article *domain.Article) error {
	return errors.ErrNotImplemented
}
func (r *articleRepo) FindByID(ctx context.Context, id string) (*domain.Article, error) {
	return nil, errors.ErrNotImplemented
}
func (r *articleRepo) ListByEmail(ctx context.Context, emailID string) ([]*domain.Article, error) {
	return nil, errors.ErrNotImplemented
}
func (r *articleRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Article, error) {
	return nil, errors.ErrNotImplemented
}
func (r *articleRepo) ListUnclustered(ctx context.Context, userID string) ([]*domain.Article, error) {
	return nil, errors.ErrNotImplemented
}

type clusterRepo struct{ s *Storage }

func (r *clusterRepo) Save(ctx context.Context, cluster *domain.Cluster) error {
	return errors.ErrNotImplemented
}
func (r *clusterRepo) FindByID(ctx context.Context, id string) (*domain.Cluster, error) {
	return nil, errors.ErrNotImplemented
}
func (r *clusterRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Cluster, error) {
	return nil, errors.ErrNotImplemented
}
func (r *clusterRepo) AddArticle(ctx context.Context, clusterID, articleID string) error {
	return errors.ErrNotImplemented
}
func (r *clusterRepo) UpdateCentroid(ctx context.Context, clusterID string, centroid []float32) error {
	return errors.ErrNotImplemented
}

type digestRepo struct{ s *Storage }

func (r *digestRepo) Save(ctx context.Context, digest *domain.Digest) error {
	return errors.ErrNotImplemented
}
func (r *digestRepo) FindByID(ctx context.Context, id string) (*domain.Digest, error) {
	return nil, errors.ErrNotImplemented
}
func (r *digestRepo) ListUnpublished(ctx context.Context, userID string) ([]*domain.Digest, error) {
	return nil, errors.ErrNotImplemented
}
func (r *digestRepo) ListRecent(ctx context.Context, userID string, limit int) ([]*domain.Digest, error) {
	return nil, errors.ErrNotImplemented
}

type embeddingRepo struct{ s *Storage }

func (r *embeddingRepo) Save(ctx context.Context, embedding *domain.Embedding) error {
	return errors.ErrNotImplemented
}
func (r *embeddingRepo) FindByID(ctx context.Context, id string) (*domain.Embedding, error) {
	return nil, errors.ErrNotImplemented
}
func (r *embeddingRepo) FindByArticleID(ctx context.Context, articleID string) (*domain.Embedding, error) {
	return nil, errors.ErrNotImplemented
}

type llmRequestRepo struct{ s *Storage }

func (r *llmRequestRepo) Save(ctx context.Context, req *domain.LLMRequest) error {
	return errors.ErrNotImplemented
}
func (r *llmRequestRepo) ListByUser(ctx context.Context, userID string, since time.Time) ([]*domain.LLMRequest, error) {
	return nil, errors.ErrNotImplemented
}
