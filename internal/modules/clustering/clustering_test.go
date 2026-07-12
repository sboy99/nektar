package clustering

import (
	"context"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

func TestCosine(t *testing.T) {
	if got := cosine([]float32{1, 0}, []float32{1, 0}); math.Abs(got-1) > 1e-9 {
		t.Fatalf("identical: got %v", got)
	}
	if got := cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(got) > 1e-9 {
		t.Fatalf("orthogonal: got %v", got)
	}
	if got := cosine([]float32{0, 0}, []float32{1, 2}); got != 0 {
		t.Fatalf("zero vector: got %v", got)
	}
	if got := cosine(nil, []float32{1}); got != 0 {
		t.Fatalf("empty: got %v", got)
	}
}

func TestHandleCreatesClusterAndPublishes(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	seedArticleWithEmbedding(t, storage, "art-1", "Alpha", []float32{1, 0, 0}, domain.StageEmbedded)

	var published []events.ClusterUpdated
	if err := bus.Subscribe(ctx, events.TopicClusterUpdated, func(_ context.Context, ev porteventbus.Event) error {
		if e, ok := ev.(events.ClusterUpdated); ok {
			published = append(published, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, Config{
		SimilarityThreshold: 0.75,
		MergeThreshold:      0.90,
	})
	if err := handler(ctx, events.EmbeddingCreated{
		UserID:      "user-1",
		EmbeddingID: "emb-art-1",
		ArticleID:   "art-1",
	}); err != nil {
		t.Fatal(err)
	}

	if len(published) != 1 {
		t.Fatalf("published = %d, want 1", len(published))
	}

	clusters, err := storage.Clusters().ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	if clusters[0].Name != "Alpha" {
		t.Fatalf("name = %q", clusters[0].Name)
	}
	if len(clusters[0].ArticleIDs) != 1 || clusters[0].ArticleIDs[0] != "art-1" {
		t.Fatalf("article ids = %+v", clusters[0].ArticleIDs)
	}

	article, err := storage.Articles().FindByID(ctx, "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if article.Stage != domain.StageClustered {
		t.Fatalf("stage = %s", article.Stage)
	}
}

func TestHandleAssignsSimilarArticle(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	seedArticleWithEmbedding(t, storage, "art-1", "First", []float32{1, 0, 0}, domain.StageEmbedded)
	seedArticleWithEmbedding(t, storage, "art-2", "Second", []float32{0.99, 0.01, 0}, domain.StageEmbedded)

	handler := Handle(logger, storage, bus, Config{
		SimilarityThreshold: 0.75,
		MergeThreshold:      0.99, // avoid merge in this test
	})

	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-1", ArticleID: "art-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-2", ArticleID: "art-2",
	}); err != nil {
		t.Fatal(err)
	}

	clusters, err := storage.Clusters().ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	if len(clusters[0].ArticleIDs) != 2 {
		t.Fatalf("article ids = %+v", clusters[0].ArticleIDs)
	}

	// Centroid should be mean of both vectors.
	want0 := float32((1 + 0.99) / 2)
	want1 := float32((0 + 0.01) / 2)
	if math.Abs(float64(clusters[0].Centroid[0]-want0)) > 1e-5 ||
		math.Abs(float64(clusters[0].Centroid[1]-want1)) > 1e-5 {
		t.Fatalf("centroid = %+v", clusters[0].Centroid)
	}
}

func TestHandleCreatesSeparateClusterWhenDissimilar(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	seedArticleWithEmbedding(t, storage, "art-1", "A", []float32{1, 0, 0}, domain.StageEmbedded)
	seedArticleWithEmbedding(t, storage, "art-2", "B", []float32{0, 1, 0}, domain.StageEmbedded)

	handler := Handle(logger, storage, bus, Config{
		SimilarityThreshold: 0.75,
		MergeThreshold:      0.90,
	})

	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-1", ArticleID: "art-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-2", ArticleID: "art-2",
	}); err != nil {
		t.Fatal(err)
	}

	clusters, err := storage.Clusters().ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 2 {
		t.Fatalf("clusters = %d, want 2", len(clusters))
	}
}

func TestHandleMergesNearDuplicateClusters(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create two clusters that are very similar but via a third article that
	// joins one and then triggers merge with the other.
	seedArticleWithEmbedding(t, storage, "art-1", "A", []float32{1, 0, 0}, domain.StageEmbedded)
	seedArticleWithEmbedding(t, storage, "art-2", "B", []float32{0.95, 0.05, 0}, domain.StageEmbedded)

	// High similarity threshold so art-2 does not join art-1's cluster;
	// low merge threshold so the two near-duplicate clusters collapse.
	handler := Handle(logger, storage, bus, Config{
		SimilarityThreshold: 0.999,
		MergeThreshold:      0.90,
	})

	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-1", ArticleID: "art-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-2", ArticleID: "art-2",
	}); err != nil {
		t.Fatal(err)
	}

	clusters, err := storage.Clusters().ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1 after merge", len(clusters))
	}
	if len(clusters[0].ArticleIDs) != 2 {
		t.Fatalf("article ids = %+v", clusters[0].ArticleIDs)
	}
}

func TestHandleIdempotentWhenAlreadyClustered(t *testing.T) {
	ctx := context.Background()
	storage := inmemory.NewStorage()
	bus := inmemory.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	seedArticleWithEmbedding(t, storage, "art-1", "Alpha", []float32{1, 0, 0}, domain.StageClustered)

	var published int
	if err := bus.Subscribe(ctx, events.TopicClusterUpdated, func(_ context.Context, _ porteventbus.Event) error {
		published++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	handler := Handle(logger, storage, bus, Config{
		SimilarityThreshold: 0.75,
		MergeThreshold:      0.90,
	})
	if err := handler(ctx, events.EmbeddingCreated{
		UserID: "user-1", EmbeddingID: "emb-art-1", ArticleID: "art-1",
	}); err != nil {
		t.Fatal(err)
	}

	if published != 0 {
		t.Fatalf("published = %d, want 0", published)
	}
	clusters, err := storage.Clusters().ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 0 {
		t.Fatalf("clusters = %d, want 0", len(clusters))
	}
}

func seedArticleWithEmbedding(
	t *testing.T,
	storage *inmemory.Storage,
	articleID, title string,
	vector []float32,
	stage domain.PipelineStage,
) {
	t.Helper()
	ctx := context.Background()

	article, err := domain.NewArticle("user-1", "email-1", title, 0)
	if err != nil {
		t.Fatal(err)
	}
	article.ID = articleID
	article.Stage = stage
	article.CreatedAt = time.Now().UTC()
	if err := storage.Articles().Save(ctx, article); err != nil {
		t.Fatal(err)
	}

	emb := &domain.Embedding{
		ID:         "emb-" + articleID,
		UserID:     "user-1",
		ArticleID:  articleID,
		Vector:     vector,
		Model:      "test",
		Provider:   "test",
		Dimensions: len(vector),
		CreatedAt:  time.Now().UTC(),
	}
	if err := storage.Embeddings().Save(ctx, emb); err != nil {
		t.Fatal(err)
	}
}
