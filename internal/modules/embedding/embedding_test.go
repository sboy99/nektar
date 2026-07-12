package embedding

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	portembedding "github.com/sboy99/nektar/internal/ports/embedding"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

type fakeEmbedder struct {
	calls atomic.Int32
	vec   []float32
}

func (f *fakeEmbedder) Embed(_ context.Context, text string) (portembedding.Result, error) {
	f.calls.Add(1)
	if text == "" {
		return portembedding.Result{}, context.Canceled
	}
	return portembedding.Result{
		Vector: f.vec,
		Tokens: 10,
		Model:  "fake-model",
	}, nil
}

func seedArticle(t *testing.T, storage *inmemory.Storage, stage domain.PipelineStage, plain, markdown string) *domain.Article {
	t.Helper()
	ctx := context.Background()
	article, err := domain.NewArticle("user-1", "email-1", "Title", 0)
	if err != nil {
		t.Fatal(err)
	}
	article.ID = "art-1"
	article.PlainText = plain
	article.Markdown = markdown
	article.Stage = stage
	if err := storage.Articles().Save(ctx, article); err != nil {
		t.Fatal(err)
	}
	return article
}

func TestHandleEmbedsAndPublishes(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &fakeEmbedder{vec: []float32{0.1, 0.2, 0.3}}

	seedArticle(t, storage, domain.StageExtracted, "Hello world article text for embedding.", "")

	var published []events.EmbeddingCreated
	if err := bus.Subscribe(ctx, events.TopicEmbeddingCreated, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.EmbeddingCreated); ok {
			published = append(published, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, embedder, cache, Config{
		Provider:        "fake",
		Model:           "fake-model",
		Dimensions:      3,
		CostPer1MTokens: 0.15,
	})

	if err := handler(ctx, events.ArticleCreated{
		UserID:    "user-1",
		ArticleID: "art-1",
		EmailID:   "email-1",
	}); err != nil {
		t.Fatal(err)
	}

	if embedder.calls.Load() != 1 {
		t.Fatalf("expected 1 embed call, got %d", embedder.calls.Load())
	}
	if len(published) != 1 {
		t.Fatalf("expected 1 EmbeddingCreated, got %d", len(published))
	}
	if published[0].ArticleID != "art-1" {
		t.Fatalf("unexpected article id: %s", published[0].ArticleID)
	}

	article, err := storage.Articles().FindByID(ctx, "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if article.Stage != domain.StageEmbedded {
		t.Fatalf("expected stage embedded, got %s", article.Stage)
	}

	emb, err := storage.Embeddings().FindByArticleID(ctx, "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(emb.Vector) != 3 {
		t.Fatalf("expected 3 dims, got %d", len(emb.Vector))
	}

	reqs, err := storage.LLMRequests().ListByUser(ctx, "user-1", time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 llm request, got %d", len(reqs))
	}
	if reqs[0].Tokens != 10 {
		t.Fatalf("expected 10 tokens, got %d", reqs[0].Tokens)
	}
}

func TestHandleIdempotent(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &fakeEmbedder{vec: []float32{0.1, 0.2}}

	seedArticle(t, storage, domain.StageEmbedded, "already done", "")

	handler := Handle(logger, storage, bus, embedder, cache, Config{Provider: "fake", Model: "fake-model"})
	if err := handler(ctx, events.ArticleCreated{UserID: "user-1", ArticleID: "art-1"}); err != nil {
		t.Fatal(err)
	}
	if embedder.calls.Load() != 0 {
		t.Fatalf("expected no embed calls, got %d", embedder.calls.Load())
	}
}

func TestHandleCacheHitSkipsProvider(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &fakeEmbedder{vec: []float32{0.5, 0.6}}

	text := "cached article body"
	seedArticle(t, storage, domain.StageExtracted, text, "")

	handler := Handle(logger, storage, bus, embedder, cache, Config{
		Provider: "fake",
		Model:    "fake-model",
	})

	ev := events.ArticleCreated{UserID: "user-1", ArticleID: "art-1", EmailID: "email-1"}
	if err := handler(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if embedder.calls.Load() != 1 {
		t.Fatalf("first run: expected 1 call, got %d", embedder.calls.Load())
	}

	article2, err := domain.NewArticle("user-1", "email-1", "Title 2", 1)
	if err != nil {
		t.Fatal(err)
	}
	article2.ID = "art-2"
	article2.PlainText = text
	article2.Stage = domain.StageExtracted
	if err := storage.Articles().Save(ctx, article2); err != nil {
		t.Fatal(err)
	}

	if err := handler(ctx, events.ArticleCreated{UserID: "user-1", ArticleID: "art-2", EmailID: "email-1"}); err != nil {
		t.Fatal(err)
	}
	if embedder.calls.Load() != 1 {
		t.Fatalf("cache hit: expected still 1 call, got %d", embedder.calls.Load())
	}

	emb, err := storage.Embeddings().FindByArticleID(ctx, "art-2")
	if err != nil {
		t.Fatal(err)
	}
	if emb.Vector[0] != 0.5 {
		t.Fatalf("expected cached vector, got %v", emb.Vector)
	}
}

func TestHandleEmptyTextFails(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &fakeEmbedder{vec: []float32{0.1}}

	seedArticle(t, storage, domain.StageExtracted, "", "")

	handler := Handle(logger, storage, bus, embedder, cache, Config{Provider: "fake", Model: "fake-model"})
	err := handler(ctx, events.ArticleCreated{UserID: "user-1", ArticleID: "art-1"})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if embedder.calls.Load() != 0 {
		t.Fatalf("expected no embed calls, got %d", embedder.calls.Load())
	}

	article, err := storage.Articles().FindByID(ctx, "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if article.Stage != domain.StageFailed {
		t.Fatalf("expected failed stage, got %s", article.Stage)
	}
}

func TestHandlePrefersPlainText(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &fakeEmbedder{vec: []float32{1}}

	seedArticle(t, storage, domain.StageExtracted, "plain body", "# markdown")

	handler := Handle(logger, storage, bus, embedder, cache, Config{Provider: "fake", Model: "fake-model"})
	if err := handler(ctx, events.ArticleCreated{UserID: "user-1", ArticleID: "art-1"}); err != nil {
		t.Fatal(err)
	}
	if embedder.calls.Load() != 1 {
		t.Fatal("expected embed call")
	}
}

func TestParseArticleCreatedMap(t *testing.T) {
	ev := mapEvent{name: events.TopicArticleCreated, payload: map[string]any{
		"UserID":    "u1",
		"ArticleID": "a1",
		"EmailID":   "e1",
	}}
	got, ok := parseArticleCreated(ev)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if got.UserID != "u1" || got.ArticleID != "a1" || got.EmailID != "e1" {
		t.Fatalf("unexpected parse: %+v", got)
	}
}

type mapEvent struct {
	name    string
	payload any
}

func (e mapEvent) Name() string { return e.name }
func (e mapEvent) Payload() any { return e.payload }
