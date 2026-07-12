package fetcher

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	portemail "github.com/sboy99/nektar/internal/ports/email"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

type stubProvider struct {
	result *portemail.FetchResult
	err    error
	calls  int
}

func (s *stubProvider) Fetch(_ context.Context, _ portemail.FetchParams) (*portemail.FetchResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *stubProvider) RefreshToken(_ context.Context, _ string) error {
	return nil
}

func TestHandleFetchesDedupesAndPublishes(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

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

	existing, err := domain.NewEmail("user-1", "dup-1", "Old", "x@y.com", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	existing.ID = "existing-id"
	if err := storage.Emails().Save(ctx, existing); err != nil {
		t.Fatal(err)
	}

	var published []events.EmailFetched
	if err := bus.Subscribe(ctx, events.TopicEmailFetched, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.EmailFetched); ok {
			published = append(published, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	newMail, _ := domain.NewEmail("user-1", "new-1", "Fresh", "n@e.com", time.Now().UTC())
	dupMail, _ := domain.NewEmail("user-1", "dup-1", "Old", "x@y.com", time.Now().UTC())
	provider := &stubProvider{result: &portemail.FetchResult{
		Emails:       []domain.Email{*newMail, *dupMail},
		NewHistoryID: "123",
	}}

	job := Handle(logger, provider, storage, bus, Config{
		DefaultQuery: "newer_than:7d",
	})
	if err := job(ctx); err != nil {
		t.Fatalf("job: %v", err)
	}

	got, err := storage.Emails().FindByGmailMessageID(ctx, "user-1", "new-1")
	if err != nil {
		t.Fatalf("find new: %v", err)
	}
	if got.ID == "" || got.Subject != "Fresh" {
		t.Fatalf("unexpected email: %+v", got)
	}

	sync, err := storage.Users().GetGmailSync(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if sync.HistoryID != "123" {
		t.Fatalf("history id: %q", sync.HistoryID)
	}
	if sync.RefreshToken != "token" {
		t.Fatalf("refresh token not preserved: %q", sync.RefreshToken)
	}
	if len(published) != 1 || published[0].EmailID != got.ID {
		t.Fatalf("published: %+v", published)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls: %d", provider.calls)
	}
}

func TestHandleSkipsMissingToken(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_ = storage.Users().SaveGmailSync(ctx, &domain.GmailSync{
		UserID:       "user-1",
		LastSyncedAt: time.Now().UTC(),
	})

	provider := &stubProvider{result: &portemail.FetchResult{NewHistoryID: "1"}}
	job := Handle(logger, provider, storage, bus, Config{})
	if err := job(ctx); err != nil {
		t.Fatalf("job: %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called, calls=%d", provider.calls)
	}
}
