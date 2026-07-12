package email

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// FetchParams configures a single Gmail sync for one user.
type FetchParams struct {
	UserID       string
	HistoryID    string
	Query        string
	RefreshToken string
}

// FetchResult holds emails retrieved from a provider plus the new sync cursor.
type FetchResult struct {
	Emails       []domain.Email
	NewHistoryID string
}

// Provider abstracts email fetching from Gmail, Outlook, IMAP, etc.
type Provider interface {
	Fetch(ctx context.Context, params FetchParams) (*FetchResult, error)
	RefreshToken(ctx context.Context, refreshToken string) error
}
