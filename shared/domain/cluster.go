package domain

import "time"

// Cluster groups related articles by semantic similarity.
type Cluster struct {
	ID         string
	UserID     string
	Name       string
	Centroid   []float32
	ArticleIDs []string
	TopicID    string
	UpdatedAt  time.Time
}
