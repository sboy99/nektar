package domain

import "time"

// Article represents extracted content from a newsletter email.
type Article struct {
	ID        string
	EmailID   string
	Title     string
	Content   string
	URL       string
	CreatedAt time.Time
}

// Digest is a curated summary of clustered articles.
type Digest struct {
	ID         string
	Title      string
	Summary    string
	ArticleIDs []string
	CreatedAt  time.Time
}

// Email represents a fetched newsletter email.
type Email struct {
	ID         string
	Subject    string
	From       string
	Body       string
	ReceivedAt time.Time
}

// Embedding is a vector representation of an article.
type Embedding struct {
	ID        string
	ArticleID string
	Vector    []float32
	CreatedAt time.Time
}

// Cluster groups related articles by semantic similarity.
type Cluster struct {
	ID         string
	Name       string
	ArticleIDs []string
	Centroid   []float32
	UpdatedAt  time.Time
}
