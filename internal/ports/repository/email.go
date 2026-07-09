package repository

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// EmailRepository persists fetched emails.
type EmailRepository interface {
	Save(ctx context.Context, email *domain.Email) error
	FindByID(ctx context.Context, id string) (*domain.Email, error)
	FindByGmailMessageID(ctx context.Context, userID, gmailMessageID string) (*domain.Email, error)
	ListUnprocessed(ctx context.Context, userID string) ([]*domain.Email, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]*domain.Email, error)
}
