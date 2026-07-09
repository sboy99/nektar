package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// ClusterRepository persists article clusters.
type ClusterRepository interface {
	Save(ctx context.Context, cluster *domain.Cluster) error
	FindByID(ctx context.Context, id string) (*domain.Cluster, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.Cluster, error)
	AddArticle(ctx context.Context, clusterID, articleID string) error
	UpdateCentroid(ctx context.Context, clusterID string, centroid []float32) error
}
