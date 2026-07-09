package summary

import "context"

// Provider generates text summaries for content.
type Provider interface {
	Summarize(ctx context.Context, text string) (string, error)
}
