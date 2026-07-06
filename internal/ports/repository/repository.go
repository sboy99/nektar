package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// ArticleRepository persists extracted articles.
type ArticleRepository interface {
	Save(ctx context.Context, article *domain.Article) error
	FindByID(ctx context.Context, id string) (*domain.Article, error)
}

// DigestRepository persists generated digests.
type DigestRepository interface {
	Save(ctx context.Context, digest *domain.Digest) error
	FindByID(ctx context.Context, id string) (*domain.Digest, error)
}

// EmailRepository persists fetched emails.
type EmailRepository interface {
	Save(ctx context.Context, email *domain.Email) error
	FindByID(ctx context.Context, id string) (*domain.Email, error)
}

// Storage groups all repository ports for convenience.
type Storage interface {
	Articles() ArticleRepository
	Digests() DigestRepository
	Emails() EmailRepository
	Close() error
}
