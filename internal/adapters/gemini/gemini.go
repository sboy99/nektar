package gemini

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	portembedding "github.com/sboy99/nektar/internal/ports/embedding"
)

// Provider implements embedding and summary ports using Google Gemini.
type Provider struct {
	client     *genai.Client
	model      string
	embedModel string
	dimensions int
}

// Config holds Gemini API configuration.
type Config struct {
	APIKey     string
	Model      string
	EmbedModel string
	Dimensions int
}

// NewProvider creates a Gemini provider that satisfies both embedding and summary ports.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	dims := cfg.Dimensions
	if dims <= 0 {
		dims = 768
	}

	return &Provider{
		client:     client,
		model:      cfg.Model,
		embedModel: cfg.EmbedModel,
		dimensions: dims,
	}, nil
}

// Embed generates a vector embedding for the given text.
func (p *Provider) Embed(ctx context.Context, text string) (portembedding.Result, error) {
	if text == "" {
		return portembedding.Result{}, fmt.Errorf("gemini: empty text")
	}

	dims := int32(p.dimensions)
	cfg := &genai.EmbedContentConfig{
		TaskType:             "RETRIEVAL_DOCUMENT",
		OutputDimensionality: &dims,
	}

	resp, err := p.client.Models.EmbedContent(ctx, p.embedModel, genai.Text(text), cfg)
	if err != nil {
		return portembedding.Result{}, fmt.Errorf("gemini: embed: %w", err)
	}
	if resp == nil || len(resp.Embeddings) == 0 || resp.Embeddings[0] == nil {
		return portembedding.Result{}, fmt.Errorf("gemini: empty embedding response")
	}

	emb := resp.Embeddings[0]
	tokens := estimateTokens(text)
	if emb.Statistics != nil && emb.Statistics.TokenCount > 0 {
		tokens = int(emb.Statistics.TokenCount)
	}

	return portembedding.Result{
		Vector: emb.Values,
		Tokens: tokens,
		Model:  p.embedModel,
	}, nil
}

func estimateTokens(text string) int {
	n := len(text) / 4
	if n < 1 {
		return 1
	}
	return n
}

// Summarize generates a summary for the given text.
func (p *Provider) Summarize(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("gemini: empty text")
	}

	model := p.model
	if model == "" {
		model = "gemini-2.0-flash"
	}

	resp, err := p.client.Models.GenerateContent(ctx, model, genai.Text(text), nil)
	if err != nil {
		return "", fmt.Errorf("gemini: summarize: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("gemini: empty summarize response")
	}

	out := strings.TrimSpace(resp.Text())
	if out == "" {
		return "", fmt.Errorf("gemini: empty summarize text")
	}
	return out, nil
}
