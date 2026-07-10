package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sboy99/nektar/shared/domain"
)

type embeddingRepo struct{ s *Storage }

func (r *embeddingRepo) Save(ctx context.Context, embedding *domain.Embedding) error {
	_, err := r.s.pool.Exec(ctx, `
		INSERT INTO embeddings (
			id, user_id, article_id, vector, model, provider, dimensions, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			article_id = EXCLUDED.article_id,
			vector = EXCLUDED.vector,
			model = EXCLUDED.model,
			provider = EXCLUDED.provider,
			dimensions = EXCLUDED.dimensions,
			created_at = EXCLUDED.created_at
	`,
		embedding.ID, embedding.UserID, embedding.ArticleID,
		float32SliceToArray(embedding.Vector), embedding.Model, embedding.Provider,
		embedding.Dimensions, embedding.CreatedAt,
	)
	return err
}

func (r *embeddingRepo) FindByID(ctx context.Context, id string) (*domain.Embedding, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, article_id, vector, model, provider, dimensions, created_at
		FROM embeddings WHERE id = $1
	`, id)
	return scanEmbedding(row)
}

func (r *embeddingRepo) FindByArticleID(ctx context.Context, articleID string) (*domain.Embedding, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, article_id, vector, model, provider, dimensions, created_at
		FROM embeddings WHERE article_id = $1
	`, articleID)
	return scanEmbedding(row)
}

func scanEmbedding(row scannable) (*domain.Embedding, error) {
	var (
		embedding domain.Embedding
		vector    pgtype.FlatArray[float32]
	)
	err := row.Scan(
		&embedding.ID, &embedding.UserID, &embedding.ArticleID, &vector,
		&embedding.Model, &embedding.Provider, &embedding.Dimensions, &embedding.CreatedAt,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	embedding.Vector = arrayToFloat32Slice(vector)
	return &embedding, nil
}
