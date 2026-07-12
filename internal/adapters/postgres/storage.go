package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sboy99/nektar/internal/ports/repository"
)

// Storage implements repository.Storage using PostgreSQL.
type Storage struct {
	pool *pgxpool.Pool
}

// NewStorage connects to PostgreSQL, runs migrations, and returns a storage adapter.
func NewStorage(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := runMigrations(dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
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

// Ping verifies the PostgreSQL connection is usable.
func (s *Storage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}
	return err
}
