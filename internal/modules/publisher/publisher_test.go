package publisher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

type fakePublisher struct {
	calls int
	err   error
}

func (f *fakePublisher) Publish(_ context.Context, _ *domain.Digest) error {
	f.calls++
	return f.err
}

func seedReadyDigest(t *testing.T, storage *inmemory.Storage, digestID string, articleIDs []string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	for _, id := range articleIDs {
		a := &domain.Article{
			ID:        id,
			UserID:    "user-1",
			EmailID:   "email-1",
			Title:     id,
			Stage:     domain.StageDigestReady,
			CreatedAt: now,
		}
		if err := storage.Articles().Save(ctx, a); err != nil {
			t.Fatal(err)
		}
	}

	dig := &domain.Digest{
		ID:                 digestID,
		UserID:             "user-1",
		Title:              "Daily Digest — 2026-07-12",
		Markdown:           "# Daily Digest\n\nHello",
		Summary:            "Key takeaways",
		ArticleIDs:         articleIDs,
		ClusterIDs:         []string{"c1"},
		ReadingTimeMinutes: 5,
		PublishStatus:      domain.PublishStatusReady,
		CreatedAt:          now,
	}
	if err := storage.Digests().Save(ctx, dig); err != nil {
		t.Fatal(err)
	}
}

func TestHandlePublishesAndMarksPublished(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pub := &fakePublisher{}

	seedReadyDigest(t, storage, "d1", []string{"a1", "a2"})

	handler := Handle(logger, storage, pub, Config{})
	if err := handler(ctx, events.DigestReady{UserID: "user-1", DigestID: "d1"}); err != nil {
		t.Fatal(err)
	}

	if pub.calls != 1 {
		t.Fatalf("publish calls = %d, want 1", pub.calls)
	}

	dig, err := storage.Digests().FindByID(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if dig.PublishStatus != domain.PublishStatusPublished {
		t.Fatalf("status = %s, want published", dig.PublishStatus)
	}
	if dig.PublishedAt == nil {
		t.Fatal("PublishedAt is nil")
	}

	for _, id := range []string{"a1", "a2"} {
		a, err := storage.Articles().FindByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Stage != domain.StagePublished {
			t.Fatalf("article %s stage = %s, want published", id, a.Stage)
		}
	}
}

func TestHandleIdempotentWhenAlreadyPublished(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pub := &fakePublisher{}

	seedReadyDigest(t, storage, "d1", []string{"a1"})
	handler := Handle(logger, storage, pub, Config{})
	if err := handler(ctx, events.DigestReady{UserID: "user-1", DigestID: "d1"}); err != nil {
		t.Fatal(err)
	}
	if err := handler(ctx, events.DigestReady{UserID: "user-1", DigestID: "d1"}); err != nil {
		t.Fatal(err)
	}
	if pub.calls != 1 {
		t.Fatalf("publish calls = %d, want 1", pub.calls)
	}
}

func TestHandlePublishErrorLeavesReady(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pub := &fakePublisher{err: errors.New("webhook down")}

	seedReadyDigest(t, storage, "d1", []string{"a1"})

	handler := Handle(logger, storage, pub, Config{})
	err := handler(ctx, events.DigestReady{UserID: "user-1", DigestID: "d1"})
	if err == nil {
		t.Fatal("expected error")
	}

	dig, findErr := storage.Digests().FindByID(ctx, "d1")
	if findErr != nil {
		t.Fatal(findErr)
	}
	if dig.PublishStatus != domain.PublishStatusReady {
		t.Fatalf("status = %s, want ready", dig.PublishStatus)
	}

	a, findErr := storage.Articles().FindByID(ctx, "a1")
	if findErr != nil {
		t.Fatal(findErr)
	}
	if a.Stage != domain.StageDigestReady {
		t.Fatalf("article stage = %s, want digest_ready", a.Stage)
	}
}

func TestHandleSkipsNonReady(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pub := &fakePublisher{}

	dig := &domain.Digest{
		ID:            "d1",
		UserID:        "user-1",
		Title:         "Draft",
		PublishStatus: domain.PublishStatusDraft,
		CreatedAt:     time.Now().UTC(),
	}
	if err := storage.Digests().Save(ctx, dig); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, pub, Config{})
	if err := handler(ctx, events.DigestReady{UserID: "user-1", DigestID: "d1"}); err != nil {
		t.Fatal(err)
	}
	if pub.calls != 0 {
		t.Fatalf("publish calls = %d, want 0", pub.calls)
	}
}

func TestParseDigestReadyMapPayload(t *testing.T) {
	ready, ok := parseDigestReady(mapEvent{
		name: events.TopicDigestReady,
		payload: map[string]any{
			"UserID":   "user-1",
			"DigestID": "d1",
		},
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if ready.UserID != "user-1" || ready.DigestID != "d1" {
		t.Fatalf("got %+v", ready)
	}
}

type mapEvent struct {
	name    string
	payload any
}

func (e mapEvent) Name() string { return e.name }
func (e mapEvent) Payload() any { return e.payload }
