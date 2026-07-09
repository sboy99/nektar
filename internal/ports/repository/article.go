package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// ArticleRepository persists extracted articles.
type ArticleRepository interface {
	Save(ctx context.Context, article *domain.Article) error
	FindByID(ctx context.Context, id string) (*domain.Article, error)
	ListByEmail(ctx context.Context, emailID string) ([]*domain.Article, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.Article, error)
	ListUnclustered(ctx context.Context, userID string) ([]*domain.Article, error)
}
