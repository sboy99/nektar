package embedding

import "context"

// Result holds the vector and usage metadata from an embedding call.
type Result struct {
	Vector []float32
	Tokens int
	Model  string
}

// Provider generates vector embeddings for text content.
type Provider interface {
	Embed(ctx context.Context, text string) (Result, error)
}
