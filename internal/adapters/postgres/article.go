package postgres

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

type articleRepo struct{ s *Storage }

func (r *articleRepo) Save(ctx context.Context, article *domain.Article) error {
	_, err := r.s.pool.Exec(ctx, `
		INSERT INTO articles (
			id, user_id, email_id, title, raw_html, markdown, plain_text,
			url, reading_time_minutes, position, stage, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			email_id = EXCLUDED.email_id,
			title = EXCLUDED.title,
			raw_html = EXCLUDED.raw_html,
			markdown = EXCLUDED.markdown,
			plain_text = EXCLUDED.plain_text,
			url = EXCLUDED.url,
			reading_time_minutes = EXCLUDED.reading_time_minutes,
			position = EXCLUDED.position,
			stage = EXCLUDED.stage,
			created_at = EXCLUDED.created_at
	`,
		article.ID, article.UserID, article.EmailID, article.Title, article.RawHTML,
		article.Markdown, article.PlainText, article.URL, article.ReadingTimeMinutes,
		article.Position, string(article.Stage), article.CreatedAt,
	)
	return err
}

func (r *articleRepo) FindByID(ctx context.Context, id string) (*domain.Article, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, email_id, title, raw_html, markdown, plain_text,
			url, reading_time_minutes, position, stage, created_at
		FROM articles WHERE id = $1
	`, id)
	return scanArticle(row)
}

func (r *articleRepo) ListByEmail(ctx context.Context, emailID string) ([]*domain.Article, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, email_id, title, raw_html, markdown, plain_text,
			url, reading_time_minutes, position, stage, created_at
		FROM articles WHERE email_id = $1
		ORDER BY position ASC
	`, emailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArticles(rows)
}

func (r *articleRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Article, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, email_id, title, raw_html, markdown, plain_text,
			url, reading_time_minutes, position, stage, created_at
		FROM articles WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArticles(rows)
}

func (r *articleRepo) ListUnclustered(ctx context.Context, userID string) ([]*domain.Article, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, email_id, title, raw_html, markdown, plain_text,
			url, reading_time_minutes, position, stage, created_at
		FROM articles
		WHERE user_id = $1 AND stage <> $2
		ORDER BY created_at DESC
	`, userID, string(domain.StageClustered))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArticles(rows)
}

func scanArticle(row scannable) (*domain.Article, error) {
	var (
		article domain.Article
		stage   string
	)
	err := row.Scan(
		&article.ID, &article.UserID, &article.EmailID, &article.Title, &article.RawHTML,
		&article.Markdown, &article.PlainText, &article.URL, &article.ReadingTimeMinutes,
		&article.Position, &stage, &article.CreatedAt,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	article.Stage = domain.PipelineStage(stage)
	return &article, nil
}

func scanArticles(rows rowIterator) ([]*domain.Article, error) {
	var result []*domain.Article
	for rows.Next() {
		article, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, article)
	}
	return result, rows.Err()
}
