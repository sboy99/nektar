package postgres

import (
	"context"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

type emailRepo struct{ s *Storage }

func (r *emailRepo) Save(ctx context.Context, email *domain.Email) error {
	headers, err := headersToJSONB(email.Headers)
	if err != nil {
		return err
	}
	_, err = r.s.pool.Exec(ctx, `
		INSERT INTO emails (
			id, user_id, gmail_message_id, thread_id, subject, "from",
			raw_body, headers, received_at, is_newsletter, newsletter_score,
			list_id, list_unsubscribe, stage
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			gmail_message_id = EXCLUDED.gmail_message_id,
			thread_id = EXCLUDED.thread_id,
			subject = EXCLUDED.subject,
			"from" = EXCLUDED."from",
			raw_body = EXCLUDED.raw_body,
			headers = EXCLUDED.headers,
			received_at = EXCLUDED.received_at,
			is_newsletter = EXCLUDED.is_newsletter,
			newsletter_score = EXCLUDED.newsletter_score,
			list_id = EXCLUDED.list_id,
			list_unsubscribe = EXCLUDED.list_unsubscribe,
			stage = EXCLUDED.stage
	`,
		email.ID, email.UserID, email.GmailMessageID, email.ThreadID, email.Subject, email.From,
		email.RawBody, headers, email.ReceivedAt, email.IsNewsletter, email.NewsletterScore,
		email.ListID, email.ListUnsubscribe, string(email.Stage),
	)
	return err
}

func (r *emailRepo) FindByID(ctx context.Context, id string) (*domain.Email, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, gmail_message_id, thread_id, subject, "from",
			raw_body, headers, received_at, is_newsletter, newsletter_score,
			list_id, list_unsubscribe, stage
		FROM emails WHERE id = $1
	`, id)
	return scanEmail(row)
}

func (r *emailRepo) FindByGmailMessageID(ctx context.Context, userID, gmailMessageID string) (*domain.Email, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, user_id, gmail_message_id, thread_id, subject, "from",
			raw_body, headers, received_at, is_newsletter, newsletter_score,
			list_id, list_unsubscribe, stage
		FROM emails WHERE user_id = $1 AND gmail_message_id = $2
	`, userID, gmailMessageID)
	return scanEmail(row)
}

func (r *emailRepo) ListUnprocessed(ctx context.Context, userID string) ([]*domain.Email, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, gmail_message_id, thread_id, subject, "from",
			raw_body, headers, received_at, is_newsletter, newsletter_score,
			list_id, list_unsubscribe, stage
		FROM emails WHERE user_id = $1 AND stage = $2
	`, userID, string(domain.StageFetched))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEmails(rows)
}

func (r *emailRepo) ListByUser(ctx context.Context, userID string, limit int) ([]*domain.Email, error) {
	query := `
		SELECT id, user_id, gmail_message_id, thread_id, subject, "from",
			raw_body, headers, received_at, is_newsletter, newsletter_score,
			list_id, list_unsubscribe, stage
		FROM emails WHERE user_id = $1
		ORDER BY received_at DESC
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
	return scanEmails(rows)
}

func (r *emailRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.s.pool.Exec(ctx, `DELETE FROM emails WHERE received_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func scanEmail(row scannable) (*domain.Email, error) {
	var (
		email  domain.Email
		stage  string
		header []byte
	)
	err := row.Scan(
		&email.ID, &email.UserID, &email.GmailMessageID, &email.ThreadID, &email.Subject, &email.From,
		&email.RawBody, &header, &email.ReceivedAt, &email.IsNewsletter, &email.NewsletterScore,
		&email.ListID, &email.ListUnsubscribe, &stage,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	email.Stage = domain.PipelineStage(stage)
	email.Headers, err = jsonbToHeaders(header)
	if err != nil {
		return nil, err
	}
	return &email, nil
}

type rowIterator interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanEmails(rows rowIterator) ([]*domain.Email, error) {
	var result []*domain.Email
	for rows.Next() {
		email, err := scanEmail(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, email)
	}
	return result, rows.Err()
}
