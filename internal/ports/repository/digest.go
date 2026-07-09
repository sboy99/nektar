package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// DigestRepository persists generated digests.
type DigestRepository interface {
	Save(ctx context.Context, digest *domain.Digest) error
	FindByID(ctx context.Context, id string) (*domain.Digest, error)
	ListUnpublished(ctx context.Context, userID string) ([]*domain.Digest, error)
	ListRecent(ctx context.Context, userID string, limit int) ([]*domain.Digest, error)
}
