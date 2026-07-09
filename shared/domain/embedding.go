package domain

import "time"

// Embedding is a vector representation of an article.
type Embedding struct {
	ID         string
	UserID     string
	ArticleID  string
	Vector     []float32
	Model      string
	Provider   string
	Dimensions int
	CreatedAt  time.Time
}
