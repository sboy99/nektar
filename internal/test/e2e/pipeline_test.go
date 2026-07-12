package e2e_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	"github.com/sboy99/nektar/internal/modules/clustering"
	"github.com/sboy99/nektar/internal/modules/digest"
	"github.com/sboy99/nektar/internal/modules/embedding"
	"github.com/sboy99/nektar/internal/modules/extractor"
	"github.com/sboy99/nektar/internal/modules/fetcher"
	"github.com/sboy99/nektar/internal/modules/newsletter"
	"github.com/sboy99/nektar/internal/modules/publisher"
	portemail "github.com/sboy99/nektar/internal/ports/email"
	portembedding "github.com/sboy99/nektar/internal/ports/embedding"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

type stubEmail struct {
	result *portemail.FetchResult
	calls  int
}

func (s *stubEmail) Fetch(_ context.Context, _ portemail.FetchParams) (*portemail.FetchResult, error) {
	s.calls++
	return s.result, nil
}

func (s *stubEmail) RefreshToken(_ context.Context, _ string) error { return nil }

type fakeEmbedder struct {
	calls atomic.Int32
	vec   []float32
}

func (f *fakeEmbedder) Embed(_ context.Context, _ string) (portembedding.Result, error) {
	f.calls.Add(1)
	return portembedding.Result{Vector: f.vec, Tokens: 8, Model: "fake-model"}, nil
}

type fakeSummarizer struct{}

func (f *fakeSummarizer) Summarize(_ context.Context, text string) (string, error) {
	if strings.Contains(text, "Propose a short cluster name") || strings.Contains(text, "topic heading") {
		return "Weekly Highlights", nil
	}
	return "- Key takeaway from the newsletter", nil
}

type fakePublisher struct {
	calls atomic.Int32
}

func (f *fakePublisher) Publish(_ context.Context, _ *domain.Digest) error {
	f.calls.Add(1)
	return nil
}

func promptsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "prompts")
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestPipelineFetchToPublish(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &fakeEmbedder{vec: []float32{1, 0, 0}}
	summarizer := &fakeSummarizer{}
	pub := &fakePublisher{}

	if err := storage.Users().Save(ctx, &domain.User{
		ID: "user-1", Email: "a@b.com", Name: "A",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Users().SaveGmailSync(ctx, &domain.GmailSync{
		UserID:       "user-1",
		Query:        "label:newsletter",
		RefreshToken: "token",
		LastSyncedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	body := `
<html><body>
<h2>Alpha Story</h2>
<p>Alpha body content with enough characters to pass the minimum article length threshold for extraction.</p>
<h2>Beta Story</h2>
<p>Beta body content with enough characters to pass the minimum article length threshold for extraction.</p>
<h2>Gamma Story</h2>
<p>Gamma body content with enough characters to pass the minimum article length threshold for extraction.</p>
</body></html>`

	mail, err := domain.NewEmail("user-1", "gmail-e2e-1", "Weekly Digest", "News <news@tldr.tech>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	mail.RawBody = body
	mail.ListID = "TLDR <tldr.tech>"
	mail.ListUnsubscribe = "<mailto:unsub@tldr.tech>"

	emailProvider := &stubEmail{result: &portemail.FetchResult{
		Emails:       []domain.Email{*mail},
		NewHistoryID: "hist-1",
	}}

	if err := bus.Subscribe(ctx, events.TopicEmailFetched, newsletter.Handle(logger, storage, bus, newsletter.Config{
		ScoreThreshold: 0.5,
	})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Subscribe(ctx, events.TopicEmailDetected, extractor.Handle(logger, storage, bus, extractor.Config{
		MinArticleChars: 50,
		WordsPerMinute:  200,
	})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Subscribe(ctx, events.TopicArticleCreated, embedding.Handle(
		logger, storage, bus, embedder, cache, embedding.Config{
			Provider:   "fake",
			Model:      "fake-model",
			Dimensions: 3,
		},
	)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Subscribe(ctx, events.TopicEmbeddingCreated, clustering.Handle(
		logger, storage, bus, clustering.Config{
			SimilarityThreshold: 0.75,
			MergeThreshold:      0.90,
		},
	)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Subscribe(ctx, events.TopicClusterUpdated, digest.Handle(
		logger, storage, bus, summarizer, digest.Config{
			PromptsDir:  promptsDir(t),
			Lookback:    24 * time.Hour,
			MinArticles: 3,
			MinClusters: 1,
		},
	)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Subscribe(ctx, events.TopicDigestReady, publisher.Handle(
		logger, storage, pub, publisher.Config{},
	)); err != nil {
		t.Fatal(err)
	}

	job := fetcher.Handle(logger, emailProvider, storage, bus, fetcher.Config{
		DefaultQuery: "newer_than:7d",
	})
	if err := job(ctx); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	emails, err := storage.Emails().ListByUser(ctx, "user-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 1 {
		t.Fatalf("emails = %d, want 1", len(emails))
	}
	if !emails[0].IsNewsletter {
		t.Fatal("expected newsletter detection")
	}

	articles, err := storage.Articles().ListByEmail(ctx, emails[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 3 {
		t.Fatalf("articles = %d, want 3", len(articles))
	}
	for _, a := range articles {
		emb, err := storage.Embeddings().FindByArticleID(ctx, a.ID)
		if err != nil {
			t.Fatalf("embedding for %s: %v", a.ID, err)
		}
		if len(emb.Vector) == 0 {
			t.Fatalf("empty embedding for %s", a.ID)
		}
	}

	clusters, err := storage.Clusters().ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) == 0 {
		t.Fatal("expected at least one cluster")
	}

	if pub.calls.Load() != 1 {
		t.Fatalf("publisher calls = %d, want 1", pub.calls.Load())
	}

	recent, err := storage.Digests().ListRecent(ctx, "user-1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 {
		t.Fatalf("digests = %d, want 1", len(recent))
	}
	if recent[0].PublishStatus != domain.PublishStatusPublished {
		t.Fatalf("digest status = %s, want published", recent[0].PublishStatus)
	}

	if err := job(ctx); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if pub.calls.Load() != 1 {
		t.Fatalf("after second fetch publisher calls = %d, want 1", pub.calls.Load())
	}
	if emailProvider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", emailProvider.calls)
	}
}
