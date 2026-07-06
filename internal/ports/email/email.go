package email

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// Provider abstracts email fetching from Gmail, Outlook, IMAP, etc.
type Provider interface {
	Fetch(ctx context.Context) ([]domain.Email, error)
}
