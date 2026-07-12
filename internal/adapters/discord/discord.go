package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

const (
	maxContentLen     = 2000
	maxEmbedDescLen   = 4096
	maxEmbeds         = 10
	maxAttempts       = 3
	baseBackoff       = 200 * time.Millisecond
	defaultEmbedColor = 0x2B6CB0
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
	Content string         `json:"content,omitempty"`
	Embeds  []embedPayload `json:"embeds,omitempty"`
}

type embedPayload struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Footer      *embedFooter `json:"footer,omitempty"`
}

type embedFooter struct {
	Text string `json:"text"`
}

type retryableError struct {
	status     int
	retryAfter time.Duration
	msg        string
}

func (e *retryableError) Error() string {
	return e.msg
}

// Publish sends a digest to a Discord channel via webhook.
func (p *Publisher) Publish(ctx context.Context, digest *domain.Digest) error {
	if p.webhookURL == "" {
		return fmt.Errorf("discord webhook url is not configured")
	}
	if digest == nil {
		return fmt.Errorf("discord: digest is nil")
	}

	content := buildContent(digest)
	embeds := buildEmbeds(digest)
	return p.sendWithRetry(ctx, content, embeds)
}

// buildContent formats a short Discord message body (max 2000 chars).
func buildContent(digest *domain.Digest) string {
	var b strings.Builder
	title := digest.Title
	if title == "" {
		title = "Daily Digest"
	}
	b.WriteString("**")
	b.WriteString(title)
	b.WriteString("**")

	summary := strings.TrimSpace(digest.Summary)
	if summary != "" {
		b.WriteString("\n")
		b.WriteString(summary)
	}

	if digest.ReadingTimeMinutes > 0 {
		b.WriteString("\n")
		fmt.Fprintf(&b, "_Estimated reading time: %d min_", digest.ReadingTimeMinutes)
	}

	return truncate(b.String(), maxContentLen)
}

// buildEmbeds builds Discord embeds from the digest markdown body.
func buildEmbeds(digest *domain.Digest) []embedPayload {
	title := digest.Title
	if title == "" {
		title = "Daily Digest"
	}

	body := strings.TrimSpace(digest.Markdown)
	if body == "" {
		body = strings.TrimSpace(digest.Summary)
	}
	if body == "" {
		body = title
	}

	chunks := chunkString(body, maxEmbedDescLen, maxEmbeds)
	embeds := make([]embedPayload, 0, len(chunks))
	for i, chunk := range chunks {
		e := embedPayload{
			Description: chunk,
			Color:       defaultEmbedColor,
		}
		if i == 0 {
			e.Title = title
			if digest.ReadingTimeMinutes > 0 {
				e.Footer = &embedFooter{
					Text: fmt.Sprintf("Estimated reading time: %d min", digest.ReadingTimeMinutes),
				}
			}
		}
		embeds = append(embeds, e)
	}
	return embeds
}

func chunkString(s string, size, maxChunks int) []string {
	if s == "" {
		return nil
	}
	if size <= 0 {
		return []string{s}
	}

	var chunks []string
	for len(s) > 0 && len(chunks) < maxChunks {
		if len(s) <= size {
			chunks = append(chunks, s)
			break
		}
		if maxChunks-len(chunks) == 1 {
			chunks = append(chunks, truncate(s, size))
			break
		}
		cut := size
		if idx := strings.LastIndex(s[:size], "\n"); idx > size/2 {
			cut = idx + 1
		}
		chunks = append(chunks, s[:cut])
		s = s[cut:]
	}
	return chunks
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func (p *Publisher) sendWithRetry(ctx context.Context, content string, embeds []embedPayload) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := p.sendWebhook(ctx, content, embeds)
		if err == nil {
			return nil
		}
		lastErr = err

		re, ok := err.(*retryableError)
		if !ok || attempt == maxAttempts {
			break
		}

		backoff := max(re.retryAfter, baseBackoff*time.Duration(1<<(attempt-1)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}

func (p *Publisher) sendWebhook(ctx context.Context, content string, embeds []embedPayload) error {
	body, err := json.Marshal(webhookPayload{Content: content, Embeds: embeds})
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
		return &retryableError{msg: fmt.Sprintf("discord webhook request failed: %v", err)}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	msg := fmt.Sprintf("discord webhook returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return &retryableError{
			status:     resp.StatusCode,
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			msg:        msg,
		}
	}
	return fmt.Errorf("%s", msg)
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}
