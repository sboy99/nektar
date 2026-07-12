package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

func TestBuildContentTruncates(t *testing.T) {
	dig := &domain.Digest{
		Title:              "Daily Digest — 2026-07-12",
		Summary:            strings.Repeat("x", 2500),
		ReadingTimeMinutes: 7,
	}
	content := buildContent(dig)
	if len(content) > maxContentLen {
		t.Fatalf("content len = %d, want <= %d", len(content), maxContentLen)
	}
	if !strings.HasSuffix(content, "...") {
		t.Fatalf("expected truncated content, got suffix %q", content[len(content)-10:])
	}
}

func TestBuildEmbedsSplitsMarkdown(t *testing.T) {
	body := strings.Repeat("a\n", maxEmbedDescLen) // forces multiple chunks
	dig := &domain.Digest{
		Title:              "Title",
		Markdown:           body,
		ReadingTimeMinutes: 3,
	}
	embeds := buildEmbeds(dig)
	if len(embeds) < 2 {
		t.Fatalf("embeds = %d, want >= 2", len(embeds))
	}
	if embeds[0].Title != "Title" {
		t.Fatalf("first title = %q", embeds[0].Title)
	}
	if embeds[0].Footer == nil || embeds[0].Footer.Text == "" {
		t.Fatal("expected footer on first embed")
	}
	for i, e := range embeds {
		if len(e.Description) > maxEmbedDescLen {
			t.Fatalf("embed %d desc len = %d", i, len(e.Description))
		}
	}
	if len(embeds) > maxEmbeds {
		t.Fatalf("embeds = %d, want <= %d", len(embeds), maxEmbeds)
	}
}

func TestPublishSuccess(t *testing.T) {
	var got webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := NewPublisher(srv.URL)
	err := p.Publish(context.Background(), &domain.Digest{
		Title:              "Daily Digest",
		Summary:            "Summary line",
		Markdown:           "# Daily Digest\n\nBody",
		ReadingTimeMinutes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "Daily Digest") {
		t.Fatalf("content = %q", got.Content)
	}
	if len(got.Embeds) == 0 {
		t.Fatal("expected embeds")
	}
}

func TestPublishRetriesOn429ThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		io.Copy(io.Discard, r.Body)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := NewPublisher(srv.URL)
	p.client.Timeout = 5 * time.Second

	err := p.Publish(context.Background(), &domain.Digest{
		Title:    "Digest",
		Markdown: "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

func TestPublishPermanent400NoRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad"}`))
	}))
	defer srv.Close()

	p := NewPublisher(srv.URL)
	err := p.Publish(context.Background(), &domain.Digest{
		Title:    "Digest",
		Markdown: "body",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestPublishEmptyWebhookURL(t *testing.T) {
	p := NewPublisher("")
	err := p.Publish(context.Background(), &domain.Digest{Title: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
