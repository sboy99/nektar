package postgres

import (
	"context"

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

func (s *Storage) Articles() repository.ArticleRepository { return &articleRepo{s} }
func (s *Storage) Digests() repository.DigestRepository   { return &digestRepo{s} }
func (s *Storage) Emails() repository.EmailRepository     { return &emailRepo{s} }

func (s *Storage) Close() error {
	s.pool.Close()
	return nil
}

type articleRepo struct{ s *Storage }

func (r *articleRepo) Save(ctx context.Context, article *domain.Article) error {
	return errors.ErrNotImplemented
}

func (r *articleRepo) FindByID(ctx context.Context, id string) (*domain.Article, error) {
	return nil, errors.ErrNotImplemented
}

type digestRepo struct{ s *Storage }

func (r *digestRepo) Save(ctx context.Context, digest *domain.Digest) error {
	return errors.ErrNotImplemented
}

func (r *digestRepo) FindByID(ctx context.Context, id string) (*domain.Digest, error) {
	return nil, errors.ErrNotImplemented
}

type emailRepo struct{ s *Storage }

func (r *emailRepo) Save(ctx context.Context, email *domain.Email) error {
	return errors.ErrNotImplemented
}

func (r *emailRepo) FindByID(ctx context.Context, id string) (*domain.Email, error) {
	return nil, errors.ErrNotImplemented
}
