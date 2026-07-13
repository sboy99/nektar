package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sboy99/nektar/internal/modules/publisher"
	"github.com/sboy99/nektar/internal/platform/metrics"
	portcache "github.com/sboy99/nektar/internal/ports/cache"
	portemail "github.com/sboy99/nektar/internal/ports/email"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	portpublisher "github.com/sboy99/nektar/internal/ports/publisher"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// AllTopics is the set of pipeline topics used for DLQ replay and stream trim.
var AllTopics = []string{
	events.TopicEmailFetched,
	events.TopicEmailDetected,
	events.TopicArticleCreated,
	events.TopicEmbeddingCreated,
	events.TopicClusterUpdated,
	events.TopicDigestReady,
}

// RetryConfig configures the DLQ replay job.
type RetryConfig struct {
	LimitPerTopic int
}

// RetryFailures returns a job that replays DLQ messages onto the main streams.
func RetryFailures(
	logger *slog.Logger,
	bus porteventbus.EventBus,
	cfg RetryConfig,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		limit := cfg.LimitPerTopic
		if limit <= 0 {
			limit = 100
		}
		n, err := bus.ReplayDLQ(ctx, AllTopics, limit)
		if err != nil {
			return fmt.Errorf("retry_failures: %w", err)
		}
		logger.Info("retry_failures: replayed", "count", n)
		return nil
	}
}

// PublishConfig configures the unpublished-digest catch-up job.
type PublishConfig struct {
	Metrics *metrics.Registry
}

// PublishDigest returns a job that publishes ready unpublished digests.
func PublishDigest(
	logger *slog.Logger,
	storage repository.Storage,
	pub portpublisher.Publisher,
	cfg PublishConfig,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		syncs, err := storage.Users().ListGmailSyncs(ctx)
		if err != nil {
			return fmt.Errorf("publish_digest: list syncs: %w", err)
		}

		pubCfg := publisher.Config{Metrics: cfg.Metrics}
		var published, skipped, failed int
		for _, sync := range syncs {
			digests, listErr := storage.Digests().ListUnpublished(ctx, sync.UserID)
			if listErr != nil {
				logger.Error("publish_digest: list unpublished", "user_id", sync.UserID, "error", listErr)
				failed++
				continue
			}
			for _, dig := range digests {
				if dig.PublishStatus != domain.PublishStatusReady {
					skipped++
					continue
				}
				if pubErr := publisher.PublishDigest(ctx, logger, storage, pub, pubCfg, dig.ID); pubErr != nil {
					logger.Error("publish_digest: publish failed",
						"digest_id", dig.ID,
						"user_id", dig.UserID,
						"error", pubErr,
					)
					failed++
					continue
				}
				published++
			}
		}

		logger.Info("publish_digest: catch-up complete",
			"published", published,
			"skipped", skipped,
			"failed", failed,
		)
		if failed > 0 {
			return fmt.Errorf("publish_digest: %d failures", failed)
		}
		return nil
	}
}

// CleanupCache returns a job that purges expired cache entries.
func CleanupCache(
	logger *slog.Logger,
	cache portcache.Cache,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		n, err := cache.PurgeExpired(ctx)
		if err != nil {
			return fmt.Errorf("cleanup_cache: %w", err)
		}
		logger.Info("cleanup_cache: purged", "count", n)
		return nil
	}
}

// CleanupEventsConfig configures retention for streams and emails.
type CleanupEventsConfig struct {
	Retention time.Duration
}

// CleanupOldEvents returns a job that trims event streams and deletes old emails.
func CleanupOldEvents(
	logger *slog.Logger,
	bus porteventbus.EventBus,
	storage repository.Storage,
	cfg CleanupEventsConfig,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		retention := cfg.Retention
		if retention <= 0 {
			retention = 168 * time.Hour
		}

		trimmed, err := bus.Trim(ctx, AllTopics, retention)
		if err != nil {
			return fmt.Errorf("cleanup_old_events: trim: %w", err)
		}

		before := time.Now().UTC().Add(-retention)
		deleted, err := storage.Emails().DeleteOlderThan(ctx, before)
		if err != nil {
			return fmt.Errorf("cleanup_old_events: delete emails: %w", err)
		}

		logger.Info("cleanup_old_events: complete",
			"trimmed", trimmed,
			"emails_deleted", deleted,
			"retention", retention.String(),
		)
		return nil
	}
}

// OAuthConfig is reserved for future OAuth refresh job options.
type OAuthConfig struct{}

// RefreshOAuth returns a job that validates/refreshes Gmail OAuth tokens.
func RefreshOAuth(
	logger *slog.Logger,
	email portemail.Provider,
	storage repository.Storage,
	_ OAuthConfig,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		syncs, err := storage.Users().ListGmailSyncs(ctx)
		if err != nil {
			return fmt.Errorf("refresh_oauth: list syncs: %w", err)
		}

		var okCount, failed int
		for _, sync := range syncs {
			if sync.RefreshToken == "" {
				logger.Warn("refresh_oauth: missing refresh_token", "user_id", sync.UserID)
				failed++
				continue
			}
			if refreshErr := email.RefreshToken(ctx, sync.RefreshToken); refreshErr != nil {
				logger.Error("refresh_oauth: refresh failed",
					"user_id", sync.UserID,
					"error", refreshErr,
				)
				failed++
				continue
			}
			okCount++
		}

		logger.Info("refresh_oauth: complete", "ok", okCount, "failed", failed)
		if failed > 0 {
			return fmt.Errorf("refresh_oauth: %d failures", failed)
		}
		return nil
	}
}
