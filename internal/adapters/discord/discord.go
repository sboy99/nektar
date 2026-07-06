package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sboy99/nektar/internal/shared/errors"
	"github.com/sboy99/nektar/shared/domain"
)

// Publisher implements the publisher port using Discord webhooks.
type Publisher struct {
	webhookURL string
	client     *http.Client
}

// NewPublisher creates a Discord webhook publisher.
func NewPublisher(webhookURL string) *Publisher {
	return &Publisher{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type webhookPayload struct {
	Content string `json:"content"`
}

// Publish sends a digest to a Discord channel via webhook.
func (p *Publisher) Publish(ctx context.Context, digest *domain.Digest) error {
	if p.webhookURL == "" {
		return fmt.Errorf("discord webhook url is not configured")
	}

	_ = ctx
	_ = digest
	return errors.ErrNotImplemented
}

// sendWebhook is a helper for when Publish is implemented.
func (p *Publisher) sendWebhook(ctx context.Context, content string) error {
	body, err := json.Marshal(webhookPayload{Content: content})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}
