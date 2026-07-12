package newsletter

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

func TestHandleDetectsAndPublishes(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-1", "Weekly Digest", "News <news@tldr.tech>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-1"
	email.ListID = "TLDR <tldr.tech>"
	email.ListUnsubscribe = "<mailto:unsub@tldr.tech>"
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	var published []events.EmailDetected
	if err := bus.Subscribe(ctx, events.TopicEmailDetected, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.EmailDetected); ok {
			published = append(published, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, Config{ScoreThreshold: 0.5})
	if err := handler(ctx, events.EmailFetched{UserID: "user-1", EmailID: "email-1"}); err != nil {
		t.Fatalf("handler: %v", err)
	}

	got, err := storage.Emails().FindByID(ctx, "email-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsNewsletter || got.Stage != domain.StageDetected {
		t.Fatalf("unexpected email state: %+v", got)
	}
	if got.ListID != "tldr.tech" {
		t.Fatalf("list id not parsed: %q", got.ListID)
	}
	if len(published) != 1 || published[0].EmailID != "email-1" {
		t.Fatalf("published: %+v", published)
	}
}

func TestHandleRejectsNonNewsletter(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-2", "Hi", "Friend <friend@gmail.com>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-2"
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	var published int
	_ = bus.Subscribe(ctx, events.TopicEmailDetected, func(_ context.Context, _ porteventbus.Event) error {
		published++
		return nil
	})

	handler := Handle(logger, storage, bus, Config{ScoreThreshold: 0.5})
	if err := handler(ctx, events.EmailFetched{UserID: "user-1", EmailID: "email-2"}); err != nil {
		t.Fatalf("handler: %v", err)
	}

	got, err := storage.Emails().FindByID(ctx, "email-2")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsNewsletter || got.Stage != domain.StageRejected {
		t.Fatalf("unexpected: %+v", got)
	}
	if published != 0 {
		t.Fatalf("should not publish, got %d", published)
	}
}

func TestHandleIdempotentAlreadyProcessed(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-3", "Digest", "n@e.com", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-3"
	email.Stage = domain.StageDetected
	email.IsNewsletter = true
	email.NewsletterScore = 0.9
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	var published int
	_ = bus.Subscribe(ctx, events.TopicEmailDetected, func(_ context.Context, _ porteventbus.Event) error {
		published++
		return nil
	})

	handler := Handle(logger, storage, bus, Config{ScoreThreshold: 0.5})
	if err := handler(ctx, events.EmailFetched{UserID: "user-1", EmailID: "email-3"}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if published != 0 {
		t.Fatalf("should not re-publish, got %d", published)
	}
}

func TestHandleParsesRedisMapPayload(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-4", "Digest", "News <n@e.com>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-4"
	email.ListID = "<list.example.com>"
	email.ListUnsubscribe = "<mailto:u@example.com>"
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	var published int
	_ = bus.Subscribe(ctx, events.TopicEmailDetected, func(_ context.Context, _ porteventbus.Event) error {
		published++
		return nil
	})

	handler := Handle(logger, storage, bus, Config{ScoreThreshold: 0.5})
	ev := &mapPayloadEvent{
		name: events.TopicEmailFetched,
		payload: map[string]any{
			"UserID":  "user-1",
			"EmailID": "email-4",
		},
	}
	if err := handler(ctx, ev); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if published != 1 {
		t.Fatalf("published=%d", published)
	}
}

type mapPayloadEvent struct {
	name    string
	payload any
}

func (e *mapPayloadEvent) Name() string { return e.name }
func (e *mapPayloadEvent) Payload() any { return e.payload }

func TestHandleDenylistReject(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	email, err := domain.NewEmail("user-1", "msg-5", "Promo", "Ads <ads@spam.com>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	email.ID = "email-5"
	email.ListID = "<ads.spam.com>"
	email.ListUnsubscribe = "<mailto:u@spam.com>"
	if err := storage.Emails().Save(ctx, email); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, Config{
		ScoreThreshold: 0.5,
		Denylist:       []string{"@spam.com"},
	})
	if err := handler(ctx, events.EmailFetched{UserID: "user-1", EmailID: "email-5"}); err != nil {
		t.Fatalf("handler: %v", err)
	}

	got, err := storage.Emails().FindByID(ctx, "email-5")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsNewsletter || got.Stage != domain.StageRejected {
		t.Fatalf("unexpected: %+v", got)
	}
}
