package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
)

type clusterRepo struct{ s *Storage }

func (r *clusterRepo) Save(ctx context.Context, cluster *domain.Cluster) error {
	tx, err := r.s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO clusters (id, user_id, name, centroid, topic_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			name = EXCLUDED.name,
			centroid = EXCLUDED.centroid,
			topic_id = EXCLUDED.topic_id,
			updated_at = EXCLUDED.updated_at
	`, cluster.ID, cluster.UserID, cluster.Name, float32SliceToArray(cluster.Centroid), nullIfEmpty(cluster.TopicID), cluster.UpdatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM cluster_articles WHERE cluster_id = $1`, cluster.ID)
	if err != nil {
		return err
	}

	for _, articleID := range cluster.ArticleIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO cluster_articles (cluster_id, article_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, cluster.ID, articleID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *clusterRepo) FindByID(ctx context.Context, id string) (*domain.Cluster, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, centroid, COALESCE(topic_id, ''), updated_at
		FROM clusters WHERE id = $1
	`, id)
	cluster, err := scanCluster(row)
	if err != nil {
		return nil, err
	}
	cluster.ArticleIDs, err = r.loadArticleIDs(ctx, cluster.ID)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

func (r *clusterRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Cluster, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, name, centroid, COALESCE(topic_id, ''), updated_at
		FROM clusters WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Cluster
	for rows.Next() {
		cluster, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, cluster)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, cluster := range result {
		cluster.ArticleIDs, err = r.loadArticleIDs(ctx, cluster.ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *clusterRepo) AddArticle(ctx context.Context, clusterID, articleID string) error {
	tag, err := r.s.pool.Exec(ctx, `
		INSERT INTO cluster_articles (cluster_id, article_id)
		SELECT $1, $2
		WHERE EXISTS (SELECT 1 FROM clusters WHERE id = $1)
		ON CONFLICT DO NOTHING
	`, clusterID, articleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either cluster missing or duplicate — distinguish by checking cluster exists.
		var exists bool
		err = r.s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1)`, clusterID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return repository.ErrNotFound
		}
	}
	return nil
}

func (r *clusterRepo) UpdateCentroid(ctx context.Context, clusterID string, centroid []float32) error {
	tag, err := r.s.pool.Exec(ctx, `
		UPDATE clusters SET centroid = $2 WHERE id = $1
	`, clusterID, float32SliceToArray(centroid))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *clusterRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.s.pool.Exec(ctx, `DELETE FROM clusters WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *clusterRepo) loadArticleIDs(ctx context.Context, clusterID string) ([]string, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT article_id FROM cluster_articles WHERE cluster_id = $1 ORDER BY article_id
	`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanCluster(row scannable) (*domain.Cluster, error) {
	var (
		cluster  domain.Cluster
		centroid pgtype.FlatArray[float32]
	)
	err := row.Scan(
		&cluster.ID,
		&cluster.UserID,
		&cluster.Name,
		&centroid,
		&cluster.TopicID,
		&cluster.UpdatedAt,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	cluster.Centroid = arrayToFloat32Slice(centroid)
	return &cluster, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
