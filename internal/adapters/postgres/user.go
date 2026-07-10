package postgres

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

type userRepo struct{ s *Storage }

func (r *userRepo) Save(ctx context.Context, user *domain.User) error {
	_, err := r.s.pool.Exec(ctx, `
		INSERT INTO users (id, email, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			updated_at = EXCLUDED.updated_at
	`, user.ID, user.Email, user.Name, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *userRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, email, name, created_at, updated_at
		FROM users WHERE id = $1
	`, id)
	return scanUser(row)
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.s.pool.QueryRow(ctx, `
		SELECT id, email, name, created_at, updated_at
		FROM users WHERE email = $1
	`, email)
	return scanUser(row)
}

func (r *userRepo) SaveGmailSync(ctx context.Context, sync *domain.GmailSync) error {
	_, err := r.s.pool.Exec(ctx, `
		INSERT INTO gmail_sync (user_id, history_id, last_synced_at, query, refresh_token_ref)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			history_id = EXCLUDED.history_id,
			last_synced_at = EXCLUDED.last_synced_at,
			query = EXCLUDED.query,
			refresh_token_ref = EXCLUDED.refresh_token_ref
	`, sync.UserID, sync.HistoryID, sync.LastSyncedAt, sync.Query, sync.RefreshTokenRef)
	return err
}

func (r *userRepo) GetGmailSync(ctx context.Context, userID string) (*domain.GmailSync, error) {
	var sync domain.GmailSync
	err := r.s.pool.QueryRow(ctx, `
		SELECT user_id, history_id, last_synced_at, query, refresh_token_ref
		FROM gmail_sync WHERE user_id = $1
	`, userID).Scan(
		&sync.UserID,
		&sync.HistoryID,
		&sync.LastSyncedAt,
		&sync.Query,
		&sync.RefreshTokenRef,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &sync, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*domain.User, error) {
	var user domain.User
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &user, nil
}
