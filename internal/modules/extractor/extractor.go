package extractor

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
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds extractor runtime settings and optional metrics.
type Config struct {
	MinArticleChars int
	WordsPerMinute  int
	Metrics         *metrics.Registry
}

// Handle processes EmailDetected events through the extraction pipeline.
func Handle(
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
) porteventbus.Handler {
	if cfg.MinArticleChars <= 0 {
		cfg.MinArticleChars = 100
	}
	if cfg.WordsPerMinute <= 0 {
		cfg.WordsPerMinute = 200
	}

	return func(ctx context.Context, event porteventbus.Event) error {
		detected, ok := parseEmailDetected(event)
		if !ok {
			logger.Error("extractor: unexpected event payload", "event", event.Name())
			return nil
		}

		start := time.Now()
		err := process(ctx, logger, storage, bus, cfg, detected)
		if cfg.Metrics != nil {
			cfg.Metrics.ExtractionDuration.Observe(time.Since(start).Seconds())
		}
		if err != nil {
			if cfg.Metrics != nil {
				cfg.Metrics.ExtractionErrors.WithLabelValues("process").Inc()
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
	detected events.EmailDetected,
) error {
	email, err := storage.Emails().FindByID(ctx, detected.EmailID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logger.Warn("extractor: email not found", "email_id", detected.EmailID)
			return nil
		}
		return fmt.Errorf("extractor: find email: %w", err)
	}

	if email.Stage == domain.StageExtracted {
		logger.Debug("extractor: already extracted", "email_id", email.ID)
		return nil
	}

	existing, err := storage.Articles().ListByEmail(ctx, email.ID)
	if err != nil {
		return fmt.Errorf("extractor: list articles: %w", err)
	}
	if len(existing) > 0 {
		logger.Debug("extractor: articles already exist", "email_id", email.ID, "count", len(existing))
		if email.Stage != domain.StageExtracted {
			email.Stage = domain.StageExtracted
			if saveErr := storage.Emails().Save(ctx, email); saveErr != nil {
				return fmt.Errorf("extractor: save email stage: %w", saveErr)
			}
		}
		return nil
	}

	email.Stage = domain.StageExtracting
	if err := storage.Emails().Save(ctx, email); err != nil {
		return fmt.Errorf("extractor: save extracting stage: %w", err)
	}

	candidates, err := extractArticles(email, cfg)
	if err != nil {
		_ = markFailed(ctx, storage, email)
		return fmt.Errorf("extractor: extract: %w", err)
	}
	if len(candidates) == 0 {
		_ = markFailed(ctx, storage, email)
		if cfg.Metrics != nil {
			cfg.Metrics.ExtractionErrors.WithLabelValues("empty").Inc()
		}
		return fmt.Errorf("extractor: no content extracted from email %s", email.ID)
	}

	for _, c := range candidates {
		article, err := domain.NewArticle(email.UserID, email.ID, c.Title, c.Position)
		if err != nil {
			_ = markFailed(ctx, storage, email)
			return fmt.Errorf("extractor: new article: %w", err)
		}
		article.ID = uuid.NewString()
		article.RawHTML = c.RawHTML
		article.Markdown = c.Markdown
		article.PlainText = c.PlainText
		article.URL = c.URL
		article.ReadingTimeMinutes = ReadingTimeMinutes(c.PlainText, cfg.WordsPerMinute)

		if err := storage.Articles().Save(ctx, article); err != nil {
			_ = markFailed(ctx, storage, email)
			return fmt.Errorf("extractor: save article: %w", err)
		}

		if cfg.Metrics != nil {
			cfg.Metrics.ArticlesExtracted.Inc()
		}

		if err := bus.Publish(ctx, events.ArticleCreated{
			UserID:    article.UserID,
			ArticleID: article.ID,
			EmailID:   article.EmailID,
		}); err != nil {
			_ = markFailed(ctx, storage, email)
			return fmt.Errorf("extractor: publish ArticleCreated: %w", err)
		}
	}

	email.Stage = domain.StageExtracted
	if err := storage.Emails().Save(ctx, email); err != nil {
		return fmt.Errorf("extractor: save extracted stage: %w", err)
	}

	logger.Info("extractor: extracted",
		"email_id", email.ID,
		"articles", len(candidates),
	)
	return nil
}

func extractArticles(email *domain.Email, cfg Config) ([]ArticleCandidate, error) {
	parsed := ParseContent(email.RawBody)
	html := EnsureHTML(parsed)
	if html == "" {
		return nil, nil
	}

	normalized, err := NormalizeHTML(html)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(normalized) == "" {
		return nil, nil
	}

	return SplitArticles(normalized, SplitConfig{
		MinArticleChars: cfg.MinArticleChars,
		WordsPerMinute:  cfg.WordsPerMinute,
		FallbackTitle:   email.Subject,
	})
}

func markFailed(ctx context.Context, storage repository.Storage, email *domain.Email) error {
	email.Stage = domain.StageFailed
	return storage.Emails().Save(ctx, email)
}

func parseEmailDetected(event porteventbus.Event) (events.EmailDetected, bool) {
	if e, ok := event.(events.EmailDetected); ok {
		return e, true
	}
	switch p := event.Payload().(type) {
	case events.EmailDetected:
		return p, true
	case map[string]any:
		userID, _ := p["UserID"].(string)
		emailID, _ := p["EmailID"].(string)
		if emailID == "" {
			return events.EmailDetected{}, false
		}
		return events.EmailDetected{UserID: userID, EmailID: emailID}, true
	default:
		return events.EmailDetected{}, false
	}
}
