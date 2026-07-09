package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// EmbeddingRepository persists article embeddings.
type EmbeddingRepository interface {
	Save(ctx context.Context, embedding *domain.Embedding) error
	FindByID(ctx context.Context, id string) (*domain.Embedding, error)
	FindByArticleID(ctx context.Context, articleID string) (*domain.Embedding, error)
}
