package digest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sboy99/nektar/internal/platform/metrics"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
	portsummary "github.com/sboy99/nektar/internal/ports/summary"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds digest pipeline settings and optional metrics.
type Config struct {
	PromptsDir            string
	Lookback              time.Duration
	MinArticles           int
	MinClusters           int
	MinClusterSize        int
	MaxClusters           int
	MaxArticlesPerCluster int
	WordsPerMinute        int
	Provider              string
	Model                 string
	CostPer1MTokens       float64
	Metrics               *metrics.Registry

	// prompts overrides file loading when set (tests).
	prompts *prompts
}

// Handle processes ClusterUpdated events through the digest pipeline.
func Handle(
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	summarizer portsummary.Provider,
	cfg Config,
) porteventbus.Handler {
	cfg = applyDefaults(cfg)

	p := cfg.prompts
	if p == nil {
		loaded, err := loadPrompts(cfg.PromptsDir)
		if err != nil {
			panic(err)
		}
		p = &loaded
	}

	return func(ctx context.Context, event porteventbus.Event) error {
		updated, ok := parseClusterUpdated(event)
		if !ok {
			logger.Error("digest: unexpected event payload", "event", event.Name())
			return nil
		}

		start := time.Now()
		err := process(ctx, logger, storage, bus, summarizer, cfg, *p, updated)
		if cfg.Metrics != nil {
			cfg.Metrics.DigestDuration.Observe(time.Since(start).Seconds())
		}
		if err != nil {
			if cfg.Metrics != nil {
				cfg.Metrics.DigestErrors.WithLabelValues("process").Inc()
			}
			return err
		}
		return nil
	}
}

func applyDefaults(cfg Config) Config {
	if cfg.PromptsDir == "" {
		cfg.PromptsDir = "prompts"
	}
	if cfg.Lookback <= 0 {
		cfg.Lookback = 24 * time.Hour
	}
	if cfg.MinArticles <= 0 {
		cfg.MinArticles = 3
	}
	if cfg.MinClusters <= 0 {
		cfg.MinClusters = 1
	}
	if cfg.MinClusterSize <= 0 {
		cfg.MinClusterSize = 1
	}
	if cfg.MaxClusters <= 0 {
		cfg.MaxClusters = 10
	}
	if cfg.MaxArticlesPerCluster <= 0 {
		cfg.MaxArticlesPerCluster = 5
	}
	if cfg.WordsPerMinute <= 0 {
		cfg.WordsPerMinute = 200
	}
	if cfg.Provider == "" {
		cfg.Provider = "gemini"
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-2.0-flash"
	}
	if cfg.CostPer1MTokens <= 0 {
		cfg.CostPer1MTokens = 0.10
	}
	return cfg
}

func process(
	ctx context.Context,
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	summarizer portsummary.Provider,
	cfg Config,
	tmpls prompts,
	updated events.ClusterUpdated,
) error {
	now := time.Now().UTC()
	day := now.Format("2006-01-02")

	existing, err := findDayDigest(ctx, storage, updated.UserID, day)
	if err != nil {
		return fmt.Errorf("digest: find day digest: %w", err)
	}
	if existing != nil && (existing.PublishStatus == domain.PublishStatusReady ||
		existing.PublishStatus == domain.PublishStatusPublished) {
		logger.Debug("digest: already ready for day",
			"user_id", updated.UserID,
			"day", day,
			"digest_id", existing.ID,
			"status", existing.PublishStatus,
		)
		return nil
	}

	clusters, err := storage.Clusters().ListByUser(ctx, updated.UserID)
	if err != nil {
		return fmt.Errorf("digest: list clusters: %w", err)
	}

	articlesByCluster := make(map[string][]*domain.Article, len(clusters))
	for _, c := range clusters {
		arts := make([]*domain.Article, 0, len(c.ArticleIDs))
		for _, id := range c.ArticleIDs {
			a, findErr := storage.Articles().FindByID(ctx, id)
			if findErr != nil {
				if errors.Is(findErr, repository.ErrNotFound) {
					continue
				}
				return fmt.Errorf("digest: find article %s: %w", id, findErr)
			}
			arts = append(arts, a)
		}
		articlesByCluster[c.ID] = arts
	}

	ranked := rankAndFilter(
		clusters,
		articlesByCluster,
		now,
		cfg.Lookback,
		cfg.MinClusterSize,
		cfg.MaxClusters,
		cfg.MaxArticlesPerCluster,
	)
	if len(ranked) == 0 {
		logger.Debug("digest: no ranked clusters", "user_id", updated.UserID)
		return nil
	}

	dig := existing
	if dig == nil {
		dig = &domain.Digest{
			ID:        uuid.NewString(),
			UserID:    updated.UserID,
			CreatedAt: now,
		}
	}

	sections := make([]topicSection, 0, len(ranked))
	var allArticleIDs []string
	var allClusterIDs []string
	var allScored []scoredArticle

	for _, sc := range ranked {
		block := formatArticlesBlock(sc.Articles)
		heading := strings.TrimSpace(sc.Cluster.Name)
		if weakClusterName(sc.Cluster, sc.Articles) {
			headingPrompt := renderPrompt(tmpls.Cluster, map[string]string{
				"articles": block,
			})
			headingText, sumErr := callSummarize(ctx, summarizer, storage, cfg, updated.UserID, dig.ID, headingPrompt)
			if sumErr != nil {
				return fmt.Errorf("digest: cluster heading: %w", sumErr)
			}
			heading = cleanHeading(headingText)
		} else {
			topicPrompt := renderPrompt(tmpls.Topic, map[string]string{
				"cluster_name": heading,
				"articles":     block,
			})
			headingText, sumErr := callSummarize(ctx, summarizer, storage, cfg, updated.UserID, dig.ID, topicPrompt)
			if sumErr != nil {
				return fmt.Errorf("digest: topic heading: %w", sumErr)
			}
			if cleaned := cleanHeading(headingText); cleaned != "" {
				heading = cleaned
			}
		}
		if heading == "" {
			heading = "Topic"
		}

		summaryPrompt := renderPrompt(tmpls.Summary, map[string]string{
			"topic_name": heading,
			"articles":   block,
		})
		summaryText, sumErr := callSummarize(ctx, summarizer, storage, cfg, updated.UserID, dig.ID, summaryPrompt)
		if sumErr != nil {
			return fmt.Errorf("digest: summarize topic: %w", sumErr)
		}

		sources := make([]sourceLink, 0, len(sc.Articles))
		for _, sa := range sc.Articles {
			sources = append(sources, sourceLink{Title: sa.Article.Title, URL: sa.Article.URL})
			allArticleIDs = append(allArticleIDs, sa.Article.ID)
			allScored = append(allScored, sa)
		}
		allClusterIDs = append(allClusterIDs, sc.Cluster.ID)

		sections = append(sections, topicSection{
			Heading: heading,
			Summary: strings.TrimSpace(summaryText),
			Sources: sources,
		})
	}

	readingTime := readingTimeSum(allScored, cfg.WordsPerMinute)
	title := dailyTitle(day)
	intro := buildIntro(day, len(sections), len(allArticleIDs))
	markdown := renderDigestMarkdown(tmpls.Digest, title, intro, sections, readingTime)

	digestSummary := ""
	if len(sections) > 0 {
		digestSummary = sections[0].Summary
	}

	dig.Title = title
	dig.Markdown = markdown
	dig.Summary = digestSummary
	dig.ArticleIDs = uniqueStrings(allArticleIDs)
	dig.ClusterIDs = uniqueStrings(allClusterIDs)
	dig.ReadingTimeMinutes = readingTime
	dig.PublishStatus = domain.PublishStatusDraft

	if err := dig.Validate(); err != nil {
		return fmt.Errorf("digest: validate: %w", err)
	}

	meetsThreshold := len(dig.ArticleIDs) >= cfg.MinArticles && len(dig.ClusterIDs) >= cfg.MinClusters
	if meetsThreshold {
		if err := dig.MarkReady(); err != nil {
			return fmt.Errorf("digest: mark ready: %w", err)
		}
	}

	if err := storage.Digests().Save(ctx, dig); err != nil {
		return fmt.Errorf("digest: save: %w", err)
	}

	stage := domain.StageDigesting
	if meetsThreshold {
		stage = domain.StageDigestReady
	}
	for _, id := range dig.ArticleIDs {
		a, findErr := storage.Articles().FindByID(ctx, id)
		if findErr != nil {
			continue
		}
		a.Stage = stage
		if saveErr := storage.Articles().Save(ctx, a); saveErr != nil {
			return fmt.Errorf("digest: save article stage: %w", saveErr)
		}
	}

	if meetsThreshold {
		if cfg.Metrics != nil {
			cfg.Metrics.DigestsGenerated.Inc()
		}
		if err := bus.Publish(ctx, events.DigestReady{
			UserID:   dig.UserID,
			DigestID: dig.ID,
		}); err != nil {
			return fmt.Errorf("digest: publish DigestReady: %w", err)
		}
		logger.Info("digest: ready",
			"digest_id", dig.ID,
			"user_id", dig.UserID,
			"articles", len(dig.ArticleIDs),
			"clusters", len(dig.ClusterIDs),
		)
	} else {
		logger.Info("digest: draft saved",
			"digest_id", dig.ID,
			"user_id", dig.UserID,
			"articles", len(dig.ArticleIDs),
			"clusters", len(dig.ClusterIDs),
			"min_articles", cfg.MinArticles,
			"min_clusters", cfg.MinClusters,
		)
	}

	return nil
}

func callSummarize(
	ctx context.Context,
	summarizer portsummary.Provider,
	storage repository.Storage,
	cfg Config,
	userID, digestID, prompt string,
) (string, error) {
	start := time.Now()
	text, err := summarizer.Summarize(ctx, prompt)
	latency := time.Since(start)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	tokens := estimateTokens(prompt) + estimateTokens(text)
	cost := float64(tokens) * cfg.CostPer1MTokens / 1_000_000

	req := &domain.LLMRequest{
		ID:               uuid.NewString(),
		UserID:           userID,
		ResourceType:     domain.ResourceTypeDigest,
		ResourceID:       digestID,
		Provider:         cfg.Provider,
		Model:            cfg.Model,
		Tokens:           tokens,
		LatencyMs:        latency.Milliseconds(),
		EstimatedCostUSD: cost,
		CreatedAt:        time.Now().UTC(),
	}
	if err := req.Validate(); err != nil {
		return "", fmt.Errorf("validate llm request: %w", err)
	}
	if err := storage.LLMRequests().Save(ctx, req); err != nil {
		return "", fmt.Errorf("save llm request: %w", err)
	}
	if cfg.Metrics != nil {
		cfg.Metrics.LLMLatency.WithLabelValues(cfg.Provider, "summarize").Observe(latency.Seconds())
		cfg.Metrics.LLMTokens.WithLabelValues(cfg.Provider, "summarize").Add(float64(tokens))
	}
	return text, nil
}

func findDayDigest(ctx context.Context, storage repository.Storage, userID, day string) (*domain.Digest, error) {
	recent, err := storage.Digests().ListRecent(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	title := dailyTitle(day)
	for _, d := range recent {
		if d.Title == title {
			return d, nil
		}
		if !d.CreatedAt.IsZero() && d.CreatedAt.UTC().Format("2006-01-02") == day {
			return d, nil
		}
	}
	unpublished, err := storage.Digests().ListUnpublished(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, d := range unpublished {
		if d.Title == title {
			return d, nil
		}
		if !d.CreatedAt.IsZero() && d.CreatedAt.UTC().Format("2006-01-02") == day {
			return d, nil
		}
	}
	return nil, nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func parseClusterUpdated(event porteventbus.Event) (events.ClusterUpdated, bool) {
	if e, ok := event.(events.ClusterUpdated); ok {
		return e, true
	}
	switch p := event.Payload().(type) {
	case events.ClusterUpdated:
		return p, true
	case map[string]any:
		userID, _ := p["UserID"].(string)
		clusterID, _ := p["ClusterID"].(string)
		if userID == "" && clusterID == "" {
			return events.ClusterUpdated{}, false
		}
		return events.ClusterUpdated{UserID: userID, ClusterID: clusterID}, true
	default:
		return events.ClusterUpdated{}, false
	}
}
