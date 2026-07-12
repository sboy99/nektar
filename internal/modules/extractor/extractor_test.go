package extractor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

func TestHandleExtractsAndPublishes(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-1", "Weekly Digest", "News <news@tldr.tech>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-1"
	email.Stage = domain.StageDetected
	email.RawBody = `
<html><body>
<h2>Alpha Story</h2>
<p>Alpha body content with enough characters to pass the minimum article length threshold for extraction.</p>
<p><a href="https://news.example/alpha">Read</a></p>
<h2>Beta Story</h2>
<p>Beta body content with enough characters to pass the minimum article length threshold for extraction.</p>
</body></html>`
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	var published []events.ArticleCreated
	if err := bus.Subscribe(ctx, events.TopicArticleCreated, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.ArticleCreated); ok {
			published = append(published, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, Config{MinArticleChars: 50, WordsPerMinute: 200})
	if err := handler(ctx, events.EmailDetected{UserID: "user-1", EmailID: "email-1"}); err != nil {
		t.Fatalf("handler: %v", err)
	}

	got, err := storage.Emails().FindByID(ctx, "email-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage != domain.StageExtracted {
		t.Fatalf("stage: %s", got.Stage)
	}

	articles, err := storage.Articles().ListByEmail(ctx, "email-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("articles: %d", len(articles))
	}
	if len(published) != 2 {
		t.Fatalf("published: %d", len(published))
	}
	if articles[0].Markdown == "" || articles[0].PlainText == "" {
		t.Fatalf("article content empty: %+v", articles[0])
	}
	if articles[0].ReadingTimeMinutes < 1 {
		t.Fatalf("reading time: %d", articles[0].ReadingTimeMinutes)
	}
}

func TestHandleIdempotent(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-2", "Once", "News <news@tldr.tech>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-2"
	email.Stage = domain.StageDetected
	email.RawBody = `<html><body><p>Single article body with enough text for extraction to succeed once.</p></body></html>`
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	var published int
	_ = bus.Subscribe(ctx, events.TopicArticleCreated, func(_ context.Context, _ porteventbus.Event) error {
		published++
		return nil
	})

	handler := Handle(logger, storage, bus, Config{MinArticleChars: 20})
	if err := handler(ctx, events.EmailDetected{UserID: "user-1", EmailID: "email-2"}); err != nil {
		t.Fatal(err)
	}
	if err := handler(ctx, events.EmailDetected{UserID: "user-1", EmailID: "email-2"}); err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("expected 1 publish, got %d", published)
	}
	articles, err := storage.Articles().ListByEmail(ctx, "email-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("articles: %d", len(articles))
	}
}

func TestHandleMissingEmail(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := Handle(logger, storage, bus, Config{})
	if err := handler(ctx, events.EmailDetected{UserID: "user-1", EmailID: "missing"}); err != nil {
		t.Fatalf("expected soft skip, got %v", err)
	}
}
