package domain

import "time"

// User is the aggregate root for account identity and Gmail sync state.
type User struct {
	ID        string
	Email     string
	Name      string
	GoogleID  string
	AvatarURL string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GmailSync holds incremental Gmail sync cursor state for a user.
type GmailSync struct {
	UserID       string
	HistoryID    string
	LastSyncedAt time.Time
	Query        string
	RefreshToken string
}
