package llm

import "context"

// Provider abstracts LLM capabilities for embedding and summarization.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Summarize(ctx context.Context, text string) (string, error)
}
