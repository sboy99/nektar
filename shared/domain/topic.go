package domain

import "time"

// Topic is a human-facing label for a cluster of related articles.
type Topic struct {
	ID          string
	UserID      string
	ClusterID   string
	Name        string
	Slug        string
	Description string
	CreatedAt   time.Time
}
