package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	portemail "github.com/sboy99/nektar/internal/ports/email"
	"github.com/sboy99/nektar/shared/domain"
)

type stubEmail struct {
	refreshCalls int
	refreshErr   error
}

func (s *stubEmail) Fetch(_ context.Context, _ portemail.FetchParams) (*portemail.FetchResult, error) {
	return &portemail.FetchResult{}, nil
}

func (s *stubEmail) RefreshToken(_ context.Context, _ string) error {
	s.refreshCalls++
	return s.refreshErr
}

type fakePublisher struct {
	calls int
	err   error
}

func (f *fakePublisher) Publish(_ context.Context, _ *domain.Digest) error {
	f.calls++
	return f.err
}

func TestRetryFailuresNoop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := inmemory.NewBus()
	job := RetryFailures(logger, bus, RetryConfig{LimitPerTopic: 10})
	if err := job(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPublishDigestCatchUp(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pub := &fakePublisher{}

	if err := storage.Users().Save(ctx, &domain.User{
		ID: "user-1", Email: "a@b.com", Name: "A",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Users().SaveGmailSync(ctx, &domain.GmailSync{
		UserID: "user-1", LastSyncedAt: time.Now().UTC(), RefreshTokenRef: "alice",
	}); err != nil {
		t.Fatal(err)
	}

	ready := &domain.Digest{
		ID: "d1", UserID: "user-1", Title: "Ready",
		PublishStatus: domain.PublishStatusReady, CreatedAt: time.Now().UTC(),
		ArticleIDs: []string{},
	}
	draft := &domain.Digest{
		ID: "d2", UserID: "user-1", Title: "Draft",
		PublishStatus: domain.PublishStatusDraft, CreatedAt: time.Now().UTC(),
	}
	if err := storage.Digests().Save(ctx, ready); err != nil {
		t.Fatal(err)
	}
	if err := storage.Digests().Save(ctx, draft); err != nil {
		t.Fatal(err)
	}

	job := PublishDigest(logger, storage, pub, PublishConfig{})
	if err := job(ctx); err != nil {
		t.Fatal(err)
	}
	if pub.calls != 1 {
		t.Fatalf("publish calls = %d, want 1", pub.calls)
	}

	got, err := storage.Digests().FindByID(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if got.PublishStatus != domain.PublishStatusPublished {
		t.Fatalf("status = %s, want published", got.PublishStatus)
	}
}

func TestCleanupCachePurgesExpired(t *testing.T) {
	ctx := context.Background()
	cache := inmemory.NewCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := cache.Set(ctx, "alive", []byte("1"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, "dead", []byte("2"), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	job := CleanupCache(logger, cache)
	if err := job(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := cache.Get(ctx, "alive"); err != nil {
		t.Fatalf("alive key should remain: %v", err)
	}
	// Purge removes expired; Get on expired would also miss.
	n, err := cache.PurgeExpired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 remaining expired, got %d", n)
	}
}

func TestCleanupOldEventsDeletesEmails(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	oldMail, err := domain.NewEmail("user-1", "old-1", "Old", "x@y.com", time.Now().UTC().Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	oldMail.ID = "email-old"
	if err := storage.Emails().Save(ctx, oldMail); err != nil {
		t.Fatal(err)
	}
	newMail, err := domain.NewEmail("user-1", "new-1", "New", "x@y.com", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	newMail.ID = "email-new"
	if err := storage.Emails().Save(ctx, newMail); err != nil {
		t.Fatal(err)
	}

	job := CleanupOldEvents(logger, bus, storage, CleanupEventsConfig{Retention: 24 * time.Hour})
	if err := job(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := storage.Emails().FindByID(ctx, "email-old"); err == nil {
		t.Fatal("expected old email deleted")
	}
	if _, err := storage.Emails().FindByID(ctx, "email-new"); err != nil {
		t.Fatalf("new email should remain: %v", err)
	}
}

func TestRefreshOAuth(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	email := &stubEmail{}

	if err := storage.Users().SaveGmailSync(ctx, &domain.GmailSync{
		UserID: "user-1", LastSyncedAt: time.Now().UTC(), RefreshTokenRef: "alice",
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Users().SaveGmailSync(ctx, &domain.GmailSync{
		UserID: "user-2", LastSyncedAt: time.Now().UTC(), RefreshTokenRef: "missing",
	}); err != nil {
		t.Fatal(err)
	}

	job := RefreshOAuth(logger, email, storage, OAuthConfig{
		RefreshTokens: map[string]string{"alice": "tok"},
	})
	err := job(ctx)
	if err == nil {
		t.Fatal("expected failure for missing token ref")
	}
	if email.refreshCalls != 1 {
		t.Fatalf("refresh calls = %d, want 1", email.refreshCalls)
	}
}

func TestRefreshOAuthProviderError(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	email := &stubEmail{refreshErr: errors.New("invalid_grant")}

	if err := storage.Users().SaveGmailSync(ctx, &domain.GmailSync{
		UserID: "user-1", LastSyncedAt: time.Now().UTC(), RefreshTokenRef: "alice",
	}); err != nil {
		t.Fatal(err)
	}

	job := RefreshOAuth(logger, email, storage, OAuthConfig{
		RefreshTokens: map[string]string{"alice": "tok"},
	})
	if err := job(ctx); err == nil {
		t.Fatal("expected error")
	}
}
