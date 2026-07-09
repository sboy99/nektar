package embedding

import "context"

// Provider generates vector embeddings for text content.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
