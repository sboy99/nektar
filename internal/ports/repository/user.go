package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// UserRepository persists user accounts and Gmail sync state.
type UserRepository interface {
	Save(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	SaveGmailSync(ctx context.Context, sync *domain.GmailSync) error
	GetGmailSync(ctx context.Context, userID string) (*domain.GmailSync, error)
	ListGmailSyncs(ctx context.Context) ([]*domain.GmailSync, error)
}
