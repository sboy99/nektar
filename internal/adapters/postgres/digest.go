package postgres

import (
	"context"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

type digestRepo struct{ s *Storage }

func (r *digestRepo) Save(ctx context.Context, digest *domain.Digest) error {
	_, err := r.s.pool.Exec(ctx, `
		INSERT INTO digests (
			id, user_id, title, markdown, summary, article_ids, cluster_ids,
			reading_time_minutes, publish_status, published_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			title = EXCLUDED.title,
			markdown = EXCLUDED.markdown,
			summary = EXCLUDED.summary,
			article_ids = EXCLUDED.article_ids,
			cluster_ids = EXCLUDED.cluster_ids,
			reading_time_minutes = EXCLUDED.reading_time_minutes,
			publish_status = EXCLUDED.publish_status,
			published_at = EXCLUDED.published_at,
			created_at = EXCLUDED.created_at
	`,
		digest.ID, digest.UserID, digest.Title, digest.Markdown, digest.Summary,
		stringSliceToArray(digest.ArticleIDs), stringSliceToArray(digest.ClusterIDs),
		digest.ReadingTimeMinutes, string(digest.PublishStatus), digest.PublishedAt, digest.CreatedAt,
	)
	return err
}

func (r *digestRepo) FindByID(ctx context.Context, id string) (*domain.Digest, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, title, markdown, summary, article_ids, cluster_ids,
			reading_time_minutes, publish_status, published_at, created_at
		FROM digests WHERE id = $1
	`, id)
	return scanDigest(row)
}

func (r *digestRepo) ListUnpublished(ctx context.Context, userID string) ([]*domain.Digest, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, title, markdown, summary, article_ids, cluster_ids,
			reading_time_minutes, publish_status, published_at, created_at
		FROM digests
		WHERE user_id = $1 AND publish_status <> $2
		ORDER BY created_at DESC
	`, userID, string(domain.PublishStatusPublished))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDigests(rows)
}

func (r *digestRepo) ListRecent(ctx context.Context, userID string, limit int) ([]*domain.Digest, error) {
	query := `
		SELECT id, user_id, title, markdown, summary, article_ids, cluster_ids,
			reading_time_minutes, publish_status, published_at, created_at
		FROM digests WHERE user_id = $1
		ORDER BY created_at DESC
	`
	args := []any{userID}
	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}
	rows, err := r.s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDigests(rows)
}

func scanDigest(row scannable) (*domain.Digest, error) {
	var (
		digest        domain.Digest
		status        string
		articleIDs    []string
		clusterIDs    []string
		publishedAt   *time.Time
	)
	err := row.Scan(
		&digest.ID, &digest.UserID, &digest.Title, &digest.Markdown, &digest.Summary,
		&articleIDs, &clusterIDs, &digest.ReadingTimeMinutes, &status, &publishedAt, &digest.CreatedAt,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	digest.PublishStatus = domain.PublishStatus(status)
	digest.ArticleIDs = arrayToStringSlice(articleIDs)
	digest.ClusterIDs = arrayToStringSlice(clusterIDs)
	digest.PublishedAt = publishedAt
	return &digest, nil
}

func scanDigests(rows rowIterator) ([]*domain.Digest, error) {
	var result []*domain.Digest
	for rows.Next() {
		digest, err := scanDigest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, digest)
	}
	return result, rows.Err()
}
