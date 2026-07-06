package gemini

import (
	"context"

	"google.golang.org/genai"

	"github.com/sboy99/nektar/internal/shared/errors"
)

// Provider implements the LLM port using Google Gemini.
type Provider struct {
	client     *genai.Client
	model      string
	embedModel string
}

// Config holds Gemini API configuration.
type Config struct {
	APIKey     string
	Model      string
	EmbedModel string
}

// NewProvider creates a Gemini LLM provider.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	return &Provider{
		client:     client,
		model:      cfg.Model,
		embedModel: cfg.EmbedModel,
	}, nil
}

// Embed generates a vector embedding for the given text.
func (p *Provider) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	_ = p.client
	_ = p.embedModel
	_ = text
	return nil, errors.ErrNotImplemented
}

// Summarize generates a summary for the given text.
func (p *Provider) Summarize(ctx context.Context, text string) (string, error) {
	_ = ctx
	_ = p.client
	_ = p.model
	_ = text
	return "", errors.ErrNotImplemented
}
