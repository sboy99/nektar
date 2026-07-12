package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	plog "github.com/sboy99/nektar/internal/platform/logger"
	"github.com/sboy99/nektar/internal/platform/metrics"
	portcache "github.com/sboy99/nektar/internal/ports/cache"
	portembedding "github.com/sboy99/nektar/internal/ports/embedding"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds embedding pipeline settings and optional metrics.
type Config struct {
	Provider        string
	Model           string
	Dimensions      int
	CacheTTL        time.Duration
	CostPer1MTokens float64
	Metrics         *metrics.Registry
}

// Handle processes ArticleCreated events through the embedding pipeline.
func Handle(
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	embedder portembedding.Provider,
	cache portcache.Cache,
	cfg Config,
) porteventbus.Handler {
	if cfg.Provider == "" {
		cfg.Provider = "gemini"
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-embedding-001"
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = 768
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 720 * time.Hour
	}

	return func(ctx context.Context, event porteventbus.Event) error {
		created, ok := parseArticleCreated(event)
		if !ok {
			logger.Error("embedding: unexpected event payload", "event", event.Name())
			return nil
		}
		log := plog.WithCorrelation(logger, created.CorrelationID)

		start := time.Now()
		err := process(ctx, log, storage, bus, embedder, cache, cfg, created)
		if cfg.Metrics != nil {
			cfg.Metrics.EmbeddingDuration.Observe(time.Since(start).Seconds())
		}
		if err != nil {
			if cfg.Metrics != nil {
				cfg.Metrics.EmbeddingErrors.WithLabelValues("process").Inc()
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
	embedder portembedding.Provider,
	cache portcache.Cache,
	cfg Config,
	created events.ArticleCreated,
) error {
	article, err := storage.Articles().FindByID(ctx, created.ArticleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logger.Warn("embedding: article not found", "article_id", created.ArticleID)
			return nil
		}
		return fmt.Errorf("embedding: find article: %w", err)
	}

	if article.Stage == domain.StageEmbedded {
		logger.Debug("embedding: already embedded", "article_id", article.ID)
		return nil
	}

	existing, err := storage.Embeddings().FindByArticleID(ctx, article.ID)
	if err == nil && existing != nil {
		logger.Debug("embedding: embedding already exists", "article_id", article.ID, "embedding_id", existing.ID)
		if article.Stage != domain.StageEmbedded {
			article.Stage = domain.StageEmbedded
			if saveErr := storage.Articles().Save(ctx, article); saveErr != nil {
				return fmt.Errorf("embedding: save article stage: %w", saveErr)
			}
		}
		return nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("embedding: find by article: %w", err)
	}

	article.Stage = domain.StageEmbedding
	if err := storage.Articles().Save(ctx, article); err != nil {
		return fmt.Errorf("embedding: save embedding stage: %w", err)
	}

	text := resolveText(article)
	if text == "" {
		_ = markFailed(ctx, storage, article)
		if cfg.Metrics != nil {
			cfg.Metrics.EmbeddingErrors.WithLabelValues("empty").Inc()
		}
		return fmt.Errorf("embedding: no text for article %s", article.ID)
	}

	vector, tokens, model, latency, fromCache, err := getOrCreateVector(ctx, embedder, cache, cfg, text)
	if err != nil {
		_ = markFailed(ctx, storage, article)
		if cfg.Metrics != nil {
			cfg.Metrics.EmbeddingErrors.WithLabelValues("embed").Inc()
		}
		return fmt.Errorf("embedding: generate: %w", err)
	}

	embedding := &domain.Embedding{
		ID:         uuid.NewString(),
		UserID:     article.UserID,
		ArticleID:  article.ID,
		Vector:     vector,
		Model:      model,
		Provider:   cfg.Provider,
		Dimensions: len(vector),
		CreatedAt:  time.Now().UTC(),
	}
	if err := embedding.Validate(); err != nil {
		_ = markFailed(ctx, storage, article)
		return fmt.Errorf("embedding: validate: %w", err)
	}
	if err := storage.Embeddings().Save(ctx, embedding); err != nil {
		_ = markFailed(ctx, storage, article)
		return fmt.Errorf("embedding: save embedding: %w", err)
	}

	cost := float64(tokens) * cfg.CostPer1MTokens / 1_000_000
	if fromCache {
		cost = 0
	}
	req := &domain.LLMRequest{
		ID:               uuid.NewString(),
		UserID:           article.UserID,
		ResourceType:     domain.ResourceTypeArticle,
		ResourceID:       article.ID,
		Provider:         cfg.Provider,
		Model:            model,
		Tokens:           tokens,
		LatencyMs:        latency.Milliseconds(),
		EstimatedCostUSD: cost,
		CreatedAt:        time.Now().UTC(),
	}
	if err := storage.LLMRequests().Save(ctx, req); err != nil {
		_ = markFailed(ctx, storage, article)
		return fmt.Errorf("embedding: save llm request: %w", err)
	}

	article.Stage = domain.StageEmbedded
	if err := storage.Articles().Save(ctx, article); err != nil {
		return fmt.Errorf("embedding: save embedded stage: %w", err)
	}

	if cfg.Metrics != nil {
		cfg.Metrics.EmbeddingsGenerated.Inc()
		if !fromCache {
			cfg.Metrics.LLMLatency.WithLabelValues(cfg.Provider, "embed").Observe(latency.Seconds())
			cfg.Metrics.LLMTokens.WithLabelValues(cfg.Provider, "embed").Add(float64(tokens))
		}
	}

	if err := bus.Publish(ctx, events.EmbeddingCreated{
		UserID:        article.UserID,
		EmbeddingID:   embedding.ID,
		ArticleID:     article.ID,
		CorrelationID: created.CorrelationID,
	}); err != nil {
		_ = markFailed(ctx, storage, article)
		return fmt.Errorf("embedding: publish EmbeddingCreated: %w", err)
	}

	logger.Info("embedding: embedded",
		"article_id", article.ID,
		"embedding_id", embedding.ID,
		"dimensions", embedding.Dimensions,
		"cached", fromCache,
		"tokens", tokens,
	)
	return nil
}

func getOrCreateVector(
	ctx context.Context,
	embedder portembedding.Provider,
	cache portcache.Cache,
	cfg Config,
	text string,
) (vector []float32, tokens int, model string, latency time.Duration, fromCache bool, err error) {
	model = cfg.Model
	cacheKey := cacheKey(cfg.Provider, model, text)

	if cache != nil {
		if raw, getErr := cache.Get(ctx, cacheKey); getErr == nil {
			var cached cachedEmbedding
			if unmarshalErr := json.Unmarshal(raw, &cached); unmarshalErr == nil && len(cached.Vector) > 0 {
				return cached.Vector, cached.Tokens, cached.Model, 0, true, nil
			}
		} else if !errors.Is(getErr, portcache.ErrNotFound) {
			// Non-fatal: fall through to provider.
		}
	}

	start := time.Now()
	result, err := embedder.Embed(ctx, text)
	latency = time.Since(start)
	if err != nil {
		return nil, 0, model, latency, false, err
	}
	if result.Model != "" {
		model = result.Model
	}
	tokens = result.Tokens
	vector = result.Vector

	if cache != nil {
		payload, marshalErr := json.Marshal(cachedEmbedding{
			Vector: vector,
			Tokens: tokens,
			Model:  model,
		})
		if marshalErr == nil {
			_ = cache.Set(ctx, cacheKey, payload, cfg.CacheTTL)
		}
	}

	return vector, tokens, model, latency, false, nil
}

type cachedEmbedding struct {
	Vector []float32 `json:"vector"`
	Tokens int       `json:"tokens"`
	Model  string    `json:"model"`
}

func cacheKey(provider, model, text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("embed:%s:%s:%s", provider, model, hex.EncodeToString(sum[:]))
}

func resolveText(article *domain.Article) string {
	if t := strings.TrimSpace(article.PlainText); t != "" {
		return t
	}
	return strings.TrimSpace(article.Markdown)
}

func markFailed(ctx context.Context, storage repository.Storage, article *domain.Article) error {
	article.Stage = domain.StageFailed
	return storage.Articles().Save(ctx, article)
}

func parseArticleCreated(event porteventbus.Event) (events.ArticleCreated, bool) {
	if e, ok := event.(events.ArticleCreated); ok {
		return e, true
	}
	switch p := event.Payload().(type) {
	case events.ArticleCreated:
		return p, true
	case map[string]any:
		userID, _ := p["UserID"].(string)
		articleID, _ := p["ArticleID"].(string)
		emailID, _ := p["EmailID"].(string)
		if articleID == "" {
			return events.ArticleCreated{}, false
		}
		return events.ArticleCreated{
			UserID:        userID,
			ArticleID:     articleID,
			EmailID:       emailID,
			CorrelationID: events.CorrelationIDFromMap(p),
		}, true
	default:
		return events.ArticleCreated{}, false
	}
}
