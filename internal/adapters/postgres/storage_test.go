package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
)

// Fixed UUID for postgres integration fixtures (users.id is UUID).
const testUserID = "11111111-1111-1111-1111-111111111111"

func testStorage(t *testing.T) *Storage {
	t.Helper()
	dsn := os.Getenv("NEKTAR_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("NEKTAR_POSTGRES_DSN not set")
	}
	ctx := context.Background()
	s, err := NewStorage(ctx, dsn)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	t.Cleanup(func() {
		cleanupTables(t, s)
		_ = s.Close()
	})
	cleanupTables(t, s)
	return s
}

func cleanupTables(t *testing.T, s *Storage) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"llm_requests", "digests", "embeddings", "cluster_articles",
		"clusters", "articles", "emails", "gmail_sync", "users",
	}
	for _, table := range tables {
		if _, err := s.pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("cleanup %s: %v", table, err)
		}
	}
}

func seedUser(t *testing.T, s *Storage, id string) *domain.User {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	user := &domain.User{
		ID:        id,
		Email:     id + "@example.com",
		Name:      "Test User",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Users().Save(context.Background(), user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedEmail(t *testing.T, s *Storage, userID, id string, stage domain.PipelineStage) *domain.Email {
	t.Helper()
	email := &domain.Email{
		ID:              id,
		UserID:          userID,
		GmailMessageID:  "gmail-" + id,
		ThreadID:        "thread-1",
		Subject:         "Hello",
		From:            "news@example.com",
		RawBody:         "<p>hi</p>",
		Headers:         map[string]string{"List-ID": "newsletter"},
		ReceivedAt:      time.Now().UTC().Truncate(time.Millisecond),
		IsNewsletter:    true,
		NewsletterScore: 0.9,
		ListID:          "newsletter",
		Stage:           stage,
	}
	if err := s.Emails().Save(context.Background(), email); err != nil {
		t.Fatalf("seed email: %v", err)
	}
	return email
}

func seedArticle(t *testing.T, s *Storage, userID, emailID, id string, stage domain.PipelineStage) *domain.Article {
	t.Helper()
	article := &domain.Article{
		ID:                 id,
		UserID:             userID,
		EmailID:            emailID,
		Title:              "Article " + id,
		RawHTML:            "<p>body</p>",
		Markdown:           "body",
		PlainText:          "body",
		URL:                "https://example.com/" + id,
		ReadingTimeMinutes: 3,
		Position:           1,
		Stage:              stage,
		CreatedAt:          time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.Articles().Save(context.Background(), article); err != nil {
		t.Fatalf("seed article: %v", err)
	}
	return article
}

func TestUserRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	user := &domain.User{
		ID:        testUserID,
		Email:     "alice@example.com",
		Name:      "Alice",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Users().Save(ctx, user); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Users().FindByID(ctx, testUserID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Email != "alice@example.com" || got.Name != "Alice" {
		t.Fatalf("unexpected user: %+v", got)
	}

	byEmail, err := s.Users().FindByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if byEmail.ID != testUserID {
		t.Fatalf("FindByEmail id = %s", byEmail.ID)
	}

	user.Name = "Alice Updated"
	user.UpdatedAt = now.Add(time.Minute)
	if err := s.Users().Save(ctx, user); err != nil {
		t.Fatalf("upsert Save: %v", err)
	}
	got, err = s.Users().FindByID(ctx, testUserID)
	if err != nil {
		t.Fatalf("FindByID after upsert: %v", err)
	}
	if got.Name != "Alice Updated" {
		t.Fatalf("name not updated: %s", got.Name)
	}

	if _, err := s.Users().FindByID(ctx, "00000000-0000-0000-0000-000000000000"); err != repository.ErrNotFound {
		t.Fatalf("FindByID missing: got %v", err)
	}

	sync := &domain.GmailSync{
		UserID:       testUserID,
		HistoryID:    "hist-1",
		LastSyncedAt: now,
		Query:        "label:newsletter",
		RefreshToken: "secret-token",
	}
	if err := s.Users().SaveGmailSync(ctx, sync); err != nil {
		t.Fatalf("SaveGmailSync: %v", err)
	}
	gotSync, err := s.Users().GetGmailSync(ctx, testUserID)
	if err != nil {
		t.Fatalf("GetGmailSync: %v", err)
	}
	if gotSync.HistoryID != "hist-1" || gotSync.Query != "label:newsletter" || gotSync.RefreshToken != "secret-token" {
		t.Fatalf("unexpected sync: %+v", gotSync)
	}

	listed, err := s.Users().ListGmailSyncs(ctx)
	if err != nil {
		t.Fatalf("ListGmailSyncs: %v", err)
	}
	if len(listed) != 1 || listed[0].UserID != testUserID {
		t.Fatalf("unexpected ListGmailSyncs: %+v", listed)
	}
}

func TestEmailRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	seedUser(t, s, testUserID)

	email := seedEmail(t, s, testUserID, "email-1", domain.StageFetched)
	got, err := s.Emails().FindByID(ctx, "email-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Subject != "Hello" || got.Headers["List-ID"] != "newsletter" {
		t.Fatalf("unexpected email: %+v", got)
	}

	byGmail, err := s.Emails().FindByGmailMessageID(ctx, testUserID, email.GmailMessageID)
	if err != nil {
		t.Fatalf("FindByGmailMessageID: %v", err)
	}
	if byGmail.ID != "email-1" {
		t.Fatalf("id = %s", byGmail.ID)
	}

	seedEmail(t, s, testUserID, "email-2", domain.StageDetected)
	unprocessed, err := s.Emails().ListUnprocessed(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListUnprocessed: %v", err)
	}
	if len(unprocessed) != 1 || unprocessed[0].ID != "email-1" {
		t.Fatalf("unprocessed = %+v", unprocessed)
	}

	listed, err := s.Emails().ListByUser(ctx, testUserID, 1)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("limit not applied: %d", len(listed))
	}
}

func TestArticleRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	seedUser(t, s, testUserID)
	seedEmail(t, s, testUserID, "email-1", domain.StageExtracted)

	seedArticle(t, s, testUserID, "email-1", "art-1", domain.StageEmbedded)
	seedArticle(t, s, testUserID, "email-1", "art-2", domain.StageClustered)

	got, err := s.Articles().FindByID(ctx, "art-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Title != "Article art-1" {
		t.Fatalf("unexpected article: %+v", got)
	}

	byEmail, err := s.Articles().ListByEmail(ctx, "email-1")
	if err != nil {
		t.Fatalf("ListByEmail: %v", err)
	}
	if len(byEmail) != 2 {
		t.Fatalf("ListByEmail len = %d", len(byEmail))
	}

	unclustered, err := s.Articles().ListUnclustered(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListUnclustered: %v", err)
	}
	if len(unclustered) != 1 || unclustered[0].ID != "art-1" {
		t.Fatalf("unclustered = %+v", unclustered)
	}
}

func TestClusterRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	seedUser(t, s, testUserID)
	seedEmail(t, s, testUserID, "email-1", domain.StageExtracted)
	seedArticle(t, s, testUserID, "email-1", "art-1", domain.StageEmbedded)
	seedArticle(t, s, testUserID, "email-1", "art-2", domain.StageEmbedded)

	now := time.Now().UTC().Truncate(time.Millisecond)
	cluster := &domain.Cluster{
		ID:         "cluster-1",
		UserID:     testUserID,
		Name:       "AI News",
		Centroid:   []float32{0.1, 0.2, 0.3},
		ArticleIDs: []string{"art-1"},
		UpdatedAt:  now,
	}
	if err := s.Clusters().Save(ctx, cluster); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Clusters().FindByID(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "AI News" || len(got.ArticleIDs) != 1 || got.ArticleIDs[0] != "art-1" {
		t.Fatalf("unexpected cluster: %+v", got)
	}
	if len(got.Centroid) != 3 || got.Centroid[1] != 0.2 {
		t.Fatalf("unexpected centroid: %v", got.Centroid)
	}

	if err := s.Clusters().AddArticle(ctx, "cluster-1", "art-2"); err != nil {
		t.Fatalf("AddArticle: %v", err)
	}
	got, err = s.Clusters().FindByID(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("FindByID after AddArticle: %v", err)
	}
	if len(got.ArticleIDs) != 2 {
		t.Fatalf("article ids = %v", got.ArticleIDs)
	}

	if err := s.Clusters().UpdateCentroid(ctx, "cluster-1", []float32{1, 2}); err != nil {
		t.Fatalf("UpdateCentroid: %v", err)
	}
	got, err = s.Clusters().FindByID(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("FindByID after UpdateCentroid: %v", err)
	}
	if len(got.Centroid) != 2 || got.Centroid[0] != 1 {
		t.Fatalf("centroid = %v", got.Centroid)
	}

	if err := s.Clusters().AddArticle(ctx, "missing", "art-1"); err != repository.ErrNotFound {
		t.Fatalf("AddArticle missing cluster: got %v", err)
	}

	listed, err := s.Clusters().ListByUser(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("ListByUser len = %d", len(listed))
	}
}

func TestDigestRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	seedUser(t, s, testUserID)

	now := time.Now().UTC().Truncate(time.Millisecond)
	publishedAt := now
	draft := &domain.Digest{
		ID:                 "digest-1",
		UserID:             testUserID,
		Title:              "Daily",
		Markdown:           "# Daily",
		Summary:            "summary",
		ArticleIDs:         []string{"art-1", "art-2"},
		ClusterIDs:         []string{"cluster-1"},
		ReadingTimeMinutes: 5,
		PublishStatus:      domain.PublishStatusDraft,
		CreatedAt:          now,
	}
	if err := s.Digests().Save(ctx, draft); err != nil {
		t.Fatalf("Save draft: %v", err)
	}

	published := &domain.Digest{
		ID:            "digest-2",
		UserID:        testUserID,
		Title:         "Yesterday",
		PublishStatus: domain.PublishStatusPublished,
		PublishedAt:   &publishedAt,
		CreatedAt:     now.Add(-time.Hour),
	}
	if err := s.Digests().Save(ctx, published); err != nil {
		t.Fatalf("Save published: %v", err)
	}

	got, err := s.Digests().FindByID(ctx, "digest-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(got.ArticleIDs) != 2 || got.ArticleIDs[0] != "art-1" {
		t.Fatalf("article ids = %v", got.ArticleIDs)
	}

	unpublished, err := s.Digests().ListUnpublished(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListUnpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].ID != "digest-1" {
		t.Fatalf("unpublished = %+v", unpublished)
	}

	recent, err := s.Digests().ListRecent(ctx, testUserID, 1)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("ListRecent len = %d", len(recent))
	}
}

func TestEmbeddingRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	seedUser(t, s, testUserID)
	seedEmail(t, s, testUserID, "email-1", domain.StageExtracted)
	seedArticle(t, s, testUserID, "email-1", "art-1", domain.StageEmbedded)

	emb := &domain.Embedding{
		ID:         "emb-1",
		UserID:     testUserID,
		ArticleID:  "art-1",
		Vector:     []float32{0.5, 0.25},
		Model:      "text-embedding-004",
		Provider:   "gemini",
		Dimensions: 2,
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.Embeddings().Save(ctx, emb); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Embeddings().FindByID(ctx, "emb-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(got.Vector) != 2 || got.Vector[0] != 0.5 {
		t.Fatalf("vector = %v", got.Vector)
	}

	byArticle, err := s.Embeddings().FindByArticleID(ctx, "art-1")
	if err != nil {
		t.Fatalf("FindByArticleID: %v", err)
	}
	if byArticle.ID != "emb-1" {
		t.Fatalf("id = %s", byArticle.ID)
	}

	if _, err := s.Embeddings().FindByID(ctx, "missing"); err != repository.ErrNotFound {
		t.Fatalf("FindByID missing: got %v", err)
	}
}

func TestLLMRequestRepository(t *testing.T) {
	s := testStorage(t)
	ctx := context.Background()
	seedUser(t, s, testUserID)

	now := time.Now().UTC().Truncate(time.Millisecond)
	req := &domain.LLMRequest{
		ID:               "llm-1",
		UserID:           testUserID,
		ResourceType:     domain.ResourceTypeArticle,
		ResourceID:       "art-1",
		Provider:         "gemini",
		Model:            "gemini-2.0-flash",
		Tokens:           100,
		LatencyMs:        50,
		EstimatedCostUSD: 0.001,
		CreatedAt:        now,
	}
	if err := s.LLMRequests().Save(ctx, req); err != nil {
		t.Fatalf("Save: %v", err)
	}

	listed, err := s.LLMRequests().ListByUser(ctx, testUserID, now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(listed) != 1 || listed[0].Tokens != 100 {
		t.Fatalf("listed = %+v", listed)
	}

	empty, err := s.LLMRequests().ListByUser(ctx, testUserID, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListByUser future: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty, got %d", len(empty))
	}
}
