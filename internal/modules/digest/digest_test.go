package digest

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

type fakeSummarizer struct {
	calls []string
}

func (f *fakeSummarizer) Summarize(_ context.Context, text string) (string, error) {
	f.calls = append(f.calls, text)
	if strings.Contains(text, "Propose a short cluster name") || strings.Contains(text, "topic heading") {
		return "Polished Topic", nil
	}
	return "- Key takeaway one\n- Key takeaway two", nil
}

func testPrompts() *prompts {
	return &prompts{
		Summary: "Summarize {{topic_name}}\n{{articles}}",
		Digest:  "# {{title}}\n\n{{intro}}\n\n{{sections}}\n\n_{{reading_time}} min_",
		Cluster: "Propose a short cluster name\n{{articles}}",
		Topic:   "Propose a clear topic heading\n{{cluster_name}}\n{{articles}}",
	}
}

func TestRenderPrompt(t *testing.T) {
	got := renderPrompt("Hello {{name}}!", map[string]string{"name": "Nektar"})
	if got != "Hello Nektar!" {
		t.Fatalf("got %q", got)
	}
}

func TestRankAndFilterOrdersAndCaps(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)

	c1 := &domain.Cluster{ID: "c1", UserID: "user-1", Name: "Big"}
	c2 := &domain.Cluster{ID: "c2", UserID: "user-1", Name: "Small"}
	c3 := &domain.Cluster{ID: "c3", UserID: "user-1", Name: "Old"}

	a1 := &domain.Article{ID: "a1", Title: "A1", Stage: domain.StageClustered, CreatedAt: now, ReadingTimeMinutes: 5}
	a2 := &domain.Article{ID: "a2", Title: "A2", Stage: domain.StageClustered, CreatedAt: now, ReadingTimeMinutes: 3}
	a3 := &domain.Article{ID: "a3", Title: "A3", Stage: domain.StageClustered, CreatedAt: now, ReadingTimeMinutes: 1}
	aOld := &domain.Article{ID: "a-old", Title: "Old", Stage: domain.StageClustered, CreatedAt: old, ReadingTimeMinutes: 10}

	ranked := rankAndFilter(
		[]*domain.Cluster{c1, c2, c3},
		map[string][]*domain.Article{
			"c1": {a1, a2},
			"c2": {a3},
			"c3": {aOld},
		},
		now,
		24*time.Hour,
		1,
		2,
		5,
	)
	if len(ranked) != 2 {
		t.Fatalf("ranked = %d, want 2", len(ranked))
	}
	if ranked[0].Cluster.ID != "c1" {
		t.Fatalf("first = %s, want c1", ranked[0].Cluster.ID)
	}
	if ranked[1].Cluster.ID != "c2" {
		t.Fatalf("second = %s, want c2", ranked[1].Cluster.ID)
	}
}

func TestHandleDraftBelowThreshold(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sum := &fakeSummarizer{}

	seedClusterWithArticles(t, storage, "c1", "Named Topic", []articleSeed{
		{id: "a1", title: "One", reading: 2},
		{id: "a2", title: "Two", reading: 2},
	})

	var ready []events.DigestReady
	if err := bus.Subscribe(ctx, events.TopicDigestReady, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.DigestReady); ok {
			ready = append(ready, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, sum, Config{
		MinArticles: 3,
		MinClusters: 1,
		Lookback:    24 * time.Hour,
		prompts:     testPrompts(),
	})
	if err := handler(ctx, events.ClusterUpdated{UserID: "user-1", ClusterID: "c1"}); err != nil {
		t.Fatal(err)
	}

	if len(ready) != 0 {
		t.Fatalf("ready events = %d, want 0", len(ready))
	}
	unpublished, err := storage.Digests().ListUnpublished(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(unpublished) != 1 {
		t.Fatalf("unpublished = %d, want 1", len(unpublished))
	}
	if unpublished[0].PublishStatus != domain.PublishStatusDraft {
		t.Fatalf("status = %s", unpublished[0].PublishStatus)
	}
	if unpublished[0].Markdown == "" {
		t.Fatal("expected markdown")
	}
}

func TestHandleReadyAndPublishes(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sum := &fakeSummarizer{}

	seedClusterWithArticles(t, storage, "c1", "Named Topic", []articleSeed{
		{id: "a1", title: "One", reading: 2},
		{id: "a2", title: "Two", reading: 2},
		{id: "a3", title: "Three", reading: 2},
	})

	var ready []events.DigestReady
	if err := bus.Subscribe(ctx, events.TopicDigestReady, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.DigestReady); ok {
			ready = append(ready, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, sum, Config{
		MinArticles: 3,
		MinClusters: 1,
		Lookback:    24 * time.Hour,
		prompts:     testPrompts(),
	})
	if err := handler(ctx, events.ClusterUpdated{UserID: "user-1", ClusterID: "c1"}); err != nil {
		t.Fatal(err)
	}

	if len(ready) != 1 {
		t.Fatalf("ready events = %d, want 1", len(ready))
	}
	dig, err := storage.Digests().FindByID(ctx, ready[0].DigestID)
	if err != nil {
		t.Fatal(err)
	}
	if dig.PublishStatus != domain.PublishStatusReady {
		t.Fatalf("status = %s", dig.PublishStatus)
	}
	if dig.ReadingTimeMinutes != 6 {
		t.Fatalf("reading time = %d", dig.ReadingTimeMinutes)
	}
	if !strings.Contains(dig.Markdown, "Key takeaway") {
		t.Fatalf("markdown missing summary: %s", dig.Markdown)
	}
	if len(sum.calls) < 2 {
		t.Fatalf("summarizer calls = %d, want >= 2", len(sum.calls))
	}

	a1, err := storage.Articles().FindByID(ctx, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if a1.Stage != domain.StageDigestReady {
		t.Fatalf("article stage = %s", a1.Stage)
	}

	reqs, err := storage.LLMRequests().ListByUser(ctx, "user-1", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) == 0 {
		t.Fatal("expected llm requests")
	}
	if reqs[0].ResourceType != domain.ResourceTypeDigest {
		t.Fatalf("resource type = %s", reqs[0].ResourceType)
	}
}

func TestHandleIdempotentWhenAlreadyReady(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sum := &fakeSummarizer{}

	day := time.Now().UTC().Format("2006-01-02")
	existing := &domain.Digest{
		ID:            "digest-1",
		UserID:        "user-1",
		Title:         dailyTitle(day),
		PublishStatus: domain.PublishStatusReady,
		CreatedAt:     time.Now().UTC(),
	}
	if err := storage.Digests().Save(ctx, existing); err != nil {
		t.Fatal(err)
	}

	seedClusterWithArticles(t, storage, "c1", "Named Topic", []articleSeed{
		{id: "a1", title: "One", reading: 2},
		{id: "a2", title: "Two", reading: 2},
		{id: "a3", title: "Three", reading: 2},
	})

	var ready int
	if err := bus.Subscribe(ctx, events.TopicDigestReady, func(_ context.Context, _ porteventbus.Event) error {
		ready++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, sum, Config{
		MinArticles: 3,
		prompts:     testPrompts(),
	})
	if err := handler(ctx, events.ClusterUpdated{UserID: "user-1", ClusterID: "c1"}); err != nil {
		t.Fatal(err)
	}
	if ready != 0 {
		t.Fatalf("ready = %d, want 0", ready)
	}
	if len(sum.calls) != 0 {
		t.Fatalf("summarizer should not be called, got %d", len(sum.calls))
	}
}

func TestLoadPromptsFromDir(t *testing.T) {
	p, err := loadPrompts("../../../prompts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Summary, "{{articles}}") {
		t.Fatalf("summary template unexpected: %q", p.Summary)
	}
	if !strings.Contains(p.Digest, "{{sections}}") {
		t.Fatalf("digest template unexpected: %q", p.Digest)
	}
}

type articleSeed struct {
	id, title string
	reading   int
}

func seedClusterWithArticles(t *testing.T, storage *inmemory.Storage, clusterID, name string, arts []articleSeed) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	ids := make([]string, 0, len(arts))
	for _, a := range arts {
		article, err := domain.NewArticle("user-1", "email-1", a.title, 0)
		if err != nil {
			t.Fatal(err)
		}
		article.ID = a.id
		article.Stage = domain.StageClustered
		article.CreatedAt = now
		article.ReadingTimeMinutes = a.reading
		article.PlainText = a.title + " body content for digest tests with enough words."
		if err := storage.Articles().Save(ctx, article); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.id)
	}

	cluster := &domain.Cluster{
		ID:         clusterID,
		UserID:     "user-1",
		Name:       name,
		ArticleIDs: ids,
		UpdatedAt:  now,
	}
	if err := storage.Clusters().Save(ctx, cluster); err != nil {
		t.Fatal(err)
	}
}
