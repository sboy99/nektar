package publisher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	plog "github.com/sboy99/nektar/internal/platform/logger"
	"github.com/sboy99/nektar/internal/platform/metrics"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	portpublisher "github.com/sboy99/nektar/internal/ports/publisher"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds publisher pipeline settings and optional metrics.
type Config struct {
	Metrics *metrics.Registry
}

// Handle processes DigestReady events.
func Handle(
	logger *slog.Logger,
	storage repository.Storage,
	pub portpublisher.Publisher,
	cfg Config,
) porteventbus.Handler {
	return func(ctx context.Context, event porteventbus.Event) error {
		ready, ok := parseDigestReady(event)
		if !ok {
			logger.Error("publisher: unexpected event payload", "event", event.Name())
			return nil
		}
		log := plog.WithCorrelation(logger, ready.CorrelationID)

		start := time.Now()
		err := PublishDigest(ctx, log, storage, pub, cfg, ready.DigestID)
		if cfg.Metrics != nil {
			cfg.Metrics.PublishDuration.Observe(time.Since(start).Seconds())
		}
		if err != nil {
			if cfg.Metrics != nil {
				cfg.Metrics.PublishErrors.WithLabelValues("process").Inc()
			}
			return err
		}
		return nil
	}
}

// PublishDigest loads a digest by ID and publishes it if status is ready.
// Idempotent for already-published digests; skips non-ready digests.
func PublishDigest(
	ctx context.Context,
	logger *slog.Logger,
	storage repository.Storage,
	pub portpublisher.Publisher,
	cfg Config,
	digestID string,
) error {
	dig, err := storage.Digests().FindByID(ctx, digestID)
	if err != nil {
		return fmt.Errorf("publisher: find digest: %w", err)
	}

	if dig.PublishStatus == domain.PublishStatusPublished {
		logger.Info("publisher: digest already published",
			"digest_id", dig.ID,
			"user_id", dig.UserID,
		)
		return nil
	}

	if dig.PublishStatus != domain.PublishStatusReady {
		logger.Info("publisher: digest not ready, skipping",
			"digest_id", dig.ID,
			"user_id", dig.UserID,
			"status", dig.PublishStatus,
		)
		return nil
	}

	if err := pub.Publish(ctx, dig); err != nil {
		return fmt.Errorf("publisher: publish: %w", err)
	}

	now := time.Now().UTC()
	if err := dig.MarkPublished(now); err != nil {
		return fmt.Errorf("publisher: mark published: %w", err)
	}
	if err := storage.Digests().Save(ctx, dig); err != nil {
		return fmt.Errorf("publisher: save digest: %w", err)
	}

	for _, id := range dig.ArticleIDs {
		a, findErr := storage.Articles().FindByID(ctx, id)
		if findErr != nil {
			continue
		}
		a.Stage = domain.StagePublished
		if saveErr := storage.Articles().Save(ctx, a); saveErr != nil {
			return fmt.Errorf("publisher: save article stage: %w", saveErr)
		}
	}

	if cfg.Metrics != nil {
		cfg.Metrics.DigestsPublished.Inc()
	}

	logger.Info("publisher: digest published",
		"digest_id", dig.ID,
		"user_id", dig.UserID,
		"articles", len(dig.ArticleIDs),
	)
	return nil
}

func parseDigestReady(event porteventbus.Event) (events.DigestReady, bool) {
	if e, ok := event.(events.DigestReady); ok {
		return e, true
	}
	switch p := event.Payload().(type) {
	case events.DigestReady:
		return p, true
	case map[string]any:
		userID, _ := p["UserID"].(string)
		digestID, _ := p["DigestID"].(string)
		if userID == "" && digestID == "" {
			return events.DigestReady{}, false
		}
		return events.DigestReady{
			UserID:        userID,
			DigestID:      digestID,
			CorrelationID: events.CorrelationIDFromMap(p),
		}, true
	default:
		return events.DigestReady{}, false
	}
}
