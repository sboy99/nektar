package clustering

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	plog "github.com/sboy99/nektar/internal/platform/logger"
	"github.com/sboy99/nektar/internal/platform/metrics"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds clustering pipeline settings and optional metrics.
type Config struct {
	SimilarityThreshold float64
	MergeThreshold      float64
	Metrics             *metrics.Registry
}

// Handle processes EmbeddingCreated events through the clustering pipeline.
func Handle(
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
) porteventbus.Handler {
	if cfg.SimilarityThreshold <= 0 {
		cfg.SimilarityThreshold = 0.75
	}
	if cfg.MergeThreshold <= 0 {
		cfg.MergeThreshold = 0.90
	}

	return func(ctx context.Context, event porteventbus.Event) error {
		created, ok := parseEmbeddingCreated(event)
		if !ok {
			logger.Error("clustering: unexpected event payload", "event", event.Name())
			return nil
		}
		log := plog.WithCorrelation(logger, created.CorrelationID)

		start := time.Now()
		err := process(ctx, log, storage, bus, cfg, created)
		if cfg.Metrics != nil {
			cfg.Metrics.ClusteringDuration.Observe(time.Since(start).Seconds())
		}
		if err != nil {
			if cfg.Metrics != nil {
				cfg.Metrics.ClusteringErrors.WithLabelValues("process").Inc()
			}
			return err
		}
		return nil
	}
}

func process(
	ctx context.Context,
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
	created events.EmbeddingCreated,
) error {
	article, err := storage.Articles().FindByID(ctx, created.ArticleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logger.Warn("clustering: article not found", "article_id", created.ArticleID)
			return nil
		}
		return fmt.Errorf("clustering: find article: %w", err)
	}

	if article.Stage == domain.StageClustered {
		logger.Debug("clustering: already clustered", "article_id", article.ID)
		return nil
	}

	embedding, err := loadEmbedding(ctx, storage, created)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logger.Warn("clustering: embedding not found",
				"embedding_id", created.EmbeddingID,
				"article_id", created.ArticleID,
			)
			return nil
		}
		return fmt.Errorf("clustering: find embedding: %w", err)
	}

	clusters, err := storage.Clusters().ListByUser(ctx, article.UserID)
	if err != nil {
		return fmt.Errorf("clustering: list clusters: %w", err)
	}

	if clusterID := findClusterContaining(clusters, article.ID); clusterID != "" {
		logger.Debug("clustering: article already in cluster", "article_id", article.ID, "cluster_id", clusterID)
		article.Stage = domain.StageClustered
		if saveErr := storage.Articles().Save(ctx, article); saveErr != nil {
			return fmt.Errorf("clustering: save article stage: %w", saveErr)
		}
		return nil
	}

	clusterID, createdNew, err := assignOrCreate(ctx, storage, cfg, article, embedding, clusters)
	if err != nil {
		_ = markFailed(ctx, storage, article)
		return err
	}

	clusterID, err = mergeClusters(ctx, storage, cfg, article.UserID, clusterID)
	if err != nil {
		_ = markFailed(ctx, storage, article)
		return fmt.Errorf("clustering: merge: %w", err)
	}

	article.Stage = domain.StageClustered
	if err := storage.Articles().Save(ctx, article); err != nil {
		return fmt.Errorf("clustering: save clustered stage: %w", err)
	}

	if cfg.Metrics != nil {
		cfg.Metrics.ArticlesClustered.Inc()
		if createdNew {
			cfg.Metrics.ClustersCreated.Inc()
		}
	}

	if err := bus.Publish(ctx, events.ClusterUpdated{
		UserID:        article.UserID,
		ClusterID:     clusterID,
		CorrelationID: created.CorrelationID,
	}); err != nil {
		_ = markFailed(ctx, storage, article)
		return fmt.Errorf("clustering: publish ClusterUpdated: %w", err)
	}

	logger.Info("clustering: clustered",
		"article_id", article.ID,
		"cluster_id", clusterID,
		"created", createdNew,
	)
	return nil
}

func loadEmbedding(ctx context.Context, storage repository.Storage, created events.EmbeddingCreated) (*domain.Embedding, error) {
	if created.EmbeddingID != "" {
		emb, err := storage.Embeddings().FindByID(ctx, created.EmbeddingID)
		if err == nil {
			return emb, nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}
	return storage.Embeddings().FindByArticleID(ctx, created.ArticleID)
}

func assignOrCreate(
	ctx context.Context,
	storage repository.Storage,
	cfg Config,
	article *domain.Article,
	embedding *domain.Embedding,
	clusters []*domain.Cluster,
) (clusterID string, created bool, err error) {
	bestID, bestScore := nearestCluster(clusters, embedding.Vector)
	if bestID != "" && bestScore >= cfg.SimilarityThreshold {
		if err := storage.Clusters().AddArticle(ctx, bestID, article.ID); err != nil {
			return "", false, fmt.Errorf("clustering: add article: %w", err)
		}
		cluster, err := storage.Clusters().FindByID(ctx, bestID)
		if err != nil {
			return "", false, fmt.Errorf("clustering: find cluster after assign: %w", err)
		}
		centroid, err := recomputeCentroid(ctx, storage, cluster.ArticleIDs)
		if err != nil {
			return "", false, err
		}
		if err := storage.Clusters().UpdateCentroid(ctx, bestID, centroid); err != nil {
			return "", false, fmt.Errorf("clustering: update centroid: %w", err)
		}
		return bestID, false, nil
	}

	name := strings.TrimSpace(article.Title)
	if name == "" {
		name = "Untitled"
	}
	now := time.Now().UTC()
	cluster := &domain.Cluster{
		ID:         uuid.NewString(),
		UserID:     article.UserID,
		Name:       name,
		Centroid:   append([]float32(nil), embedding.Vector...),
		ArticleIDs: []string{article.ID},
		UpdatedAt:  now,
	}
	if err := cluster.Validate(); err != nil {
		return "", false, fmt.Errorf("clustering: validate: %w", err)
	}
	if err := storage.Clusters().Save(ctx, cluster); err != nil {
		return "", false, fmt.Errorf("clustering: save cluster: %w", err)
	}
	return cluster.ID, true, nil
}

func mergeClusters(
	ctx context.Context,
	storage repository.Storage,
	cfg Config,
	userID, preferredID string,
) (string, error) {
	for {
		clusters, err := storage.Clusters().ListByUser(ctx, userID)
		if err != nil {
			return preferredID, err
		}
		if len(clusters) < 2 {
			return preferredID, nil
		}

		i, j, _ := bestMergePair(clusters, cfg.MergeThreshold)
		if i < 0 {
			return preferredID, nil
		}

		a, b := clusters[i], clusters[j]
		target, source := a, b
		if len(b.ArticleIDs) > len(a.ArticleIDs) {
			target, source = b, a
		}

		for _, articleID := range source.ArticleIDs {
			if containsID(target.ArticleIDs, articleID) {
				continue
			}
			if err := storage.Clusters().AddArticle(ctx, target.ID, articleID); err != nil {
				return preferredID, fmt.Errorf("add article during merge: %w", err)
			}
		}

		updated, err := storage.Clusters().FindByID(ctx, target.ID)
		if err != nil {
			return preferredID, fmt.Errorf("find target after merge: %w", err)
		}
		centroid, err := recomputeCentroid(ctx, storage, updated.ArticleIDs)
		if err != nil {
			return preferredID, err
		}
		if err := storage.Clusters().UpdateCentroid(ctx, target.ID, centroid); err != nil {
			return preferredID, fmt.Errorf("update centroid during merge: %w", err)
		}
		if err := storage.Clusters().Delete(ctx, source.ID); err != nil {
			return preferredID, fmt.Errorf("delete merged cluster: %w", err)
		}

		if preferredID == source.ID {
			preferredID = target.ID
		}

		if cfg.Metrics != nil {
			cfg.Metrics.ClustersMerged.Inc()
		}
	}
}

func bestMergePair(clusters []*domain.Cluster, threshold float64) (i, j int, score float64) {
	bestI, bestJ := -1, -1
	best := threshold
	for a := 0; a < len(clusters); a++ {
		for b := a + 1; b < len(clusters); b++ {
			sim := cosine(clusters[a].Centroid, clusters[b].Centroid)
			if sim >= best {
				best = sim
				bestI, bestJ = a, b
			}
		}
	}
	return bestI, bestJ, best
}

func nearestCluster(clusters []*domain.Cluster, vector []float32) (id string, score float64) {
	best := math.Inf(-1)
	for _, c := range clusters {
		sim := cosine(vector, c.Centroid)
		if sim > best {
			best = sim
			id = c.ID
		}
	}
	if id == "" {
		return "", 0
	}
	return id, best
}

func recomputeCentroid(ctx context.Context, storage repository.Storage, articleIDs []string) ([]float32, error) {
	vectors := make([][]float32, 0, len(articleIDs))
	for _, articleID := range articleIDs {
		emb, err := storage.Embeddings().FindByArticleID(ctx, articleID)
		if err != nil {
			return nil, fmt.Errorf("clustering: embedding for article %s: %w", articleID, err)
		}
		vectors = append(vectors, emb.Vector)
	}
	centroid := meanCentroid(vectors)
	if centroid == nil {
		return nil, fmt.Errorf("clustering: empty centroid")
	}
	return centroid, nil
}

func meanCentroid(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	dims := len(vectors[0])
	if dims == 0 {
		return nil
	}
	sum := make([]float32, dims)
	n := 0
	for _, v := range vectors {
		if len(v) != dims {
			continue
		}
		for i, x := range v {
			sum[i] += x
		}
		n++
	}
	if n == 0 {
		return nil
	}
	inv := 1 / float32(n)
	for i := range sum {
		sum[i] *= inv
	}
	return sum
}

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func findClusterContaining(clusters []*domain.Cluster, articleID string) string {
	for _, c := range clusters {
		if containsID(c.ArticleIDs, articleID) {
			return c.ID
		}
	}
	return ""
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func markFailed(ctx context.Context, storage repository.Storage, article *domain.Article) error {
	article.Stage = domain.StageFailed
	return storage.Articles().Save(ctx, article)
}

func parseEmbeddingCreated(event porteventbus.Event) (events.EmbeddingCreated, bool) {
	if e, ok := event.(events.EmbeddingCreated); ok {
		return e, true
	}
	switch p := event.Payload().(type) {
	case events.EmbeddingCreated:
		return p, true
	case map[string]any:
		userID, _ := p["UserID"].(string)
		embeddingID, _ := p["EmbeddingID"].(string)
		articleID, _ := p["ArticleID"].(string)
		if articleID == "" && embeddingID == "" {
			return events.EmbeddingCreated{}, false
		}
		return events.EmbeddingCreated{
			UserID:        userID,
			EmbeddingID:   embeddingID,
			ArticleID:     articleID,
			CorrelationID: events.CorrelationIDFromMap(p),
		}, true
	default:
		return events.EmbeddingCreated{}, false
	}
}
