package fetcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/sboy99/nektar/internal/platform/logger"
	"github.com/sboy99/nektar/internal/platform/metrics"
	"github.com/sboy99/nektar/internal/platform/retry"
	portemail "github.com/sboy99/nektar/internal/ports/email"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds fetcher runtime dependencies beyond ports.
type Config struct {
	DefaultQuery string
	Metrics      *metrics.Registry
	Retry        retry.Policy
}

// Handle returns a scheduler job function that fetches emails for all users.
func Handle(
	logger *slog.Logger,
	email portemail.Provider,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		start := time.Now()
		defer func() {
			if cfg.Metrics != nil {
				cfg.Metrics.FetchDuration.Observe(time.Since(start).Seconds())
			}
		}()

		syncs, err := storage.Users().ListGmailSyncs(ctx)
		if err != nil {
			incFetchError(cfg.Metrics, "list_syncs")
			logger.Error("fetcher: list gmail syncs", "error", err)
			return nil
		}
		if len(syncs) == 0 {
			logger.Info("fetcher: no gmail syncs configured")
			return nil
		}

		var failures int
		for _, sync := range syncs {
			if err := syncUser(ctx, logger, email, storage, bus, cfg, sync); err != nil {
				failures++
				logger.Error("fetcher: user sync failed", "user_id", sync.UserID, "error", err)
			}
		}

		if failures == len(syncs) {
			logger.Error("fetcher: all user syncs failed", "count", failures)
		}
		return nil
	}
}

func syncUser(
	ctx context.Context,
	log *slog.Logger,
	email portemail.Provider,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
	sync *domain.GmailSync,
) error {
	if sync.RefreshToken == "" {
		incFetchError(cfg.Metrics, "missing_token")
		return fmt.Errorf("missing refresh_token")
	}

	query := sync.Query
	if query == "" {
		query = cfg.DefaultQuery
	}

	result, err := fetchWithRetry(ctx, cfg.Retry, email, portemail.FetchParams{
		UserID:       sync.UserID,
		HistoryID:    sync.HistoryID,
		Query:        query,
		RefreshToken: sync.RefreshToken,
	})
	if err != nil {
		incFetchError(cfg.Metrics, "provider")
		return err
	}

	for i := range result.Emails {
		raw := &result.Emails[i]
		if err := persistEmail(ctx, log, storage, bus, cfg, raw); err != nil {
			incFetchError(cfg.Metrics, "persist")
			return err
		}
	}

	updated := &domain.GmailSync{
		UserID:       sync.UserID,
		HistoryID:    result.NewHistoryID,
		LastSyncedAt: time.Now().UTC(),
		Query:        sync.Query,
		RefreshToken: sync.RefreshToken,
	}
	if err := storage.Users().SaveGmailSync(ctx, updated); err != nil {
		incFetchError(cfg.Metrics, "save_sync")
		return fmt.Errorf("save gmail sync: %w", err)
	}

	log.Info("fetcher: user sync complete",
		"user_id", sync.UserID,
		"fetched", len(result.Emails),
		"history_id", result.NewHistoryID,
	)
	return nil
}

func persistEmail(
	ctx context.Context,
	log *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
	raw *domain.Email,
) error {
	existing, err := storage.Emails().FindByGmailMessageID(ctx, raw.UserID, raw.GmailMessageID)
	if err == nil && existing != nil {
		if cfg.Metrics != nil {
			cfg.Metrics.EmailsDeduplicated.Inc()
		}
		return nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("dedupe lookup: %w", err)
	}

	email, err := domain.NewEmail(raw.UserID, raw.GmailMessageID, raw.Subject, raw.From, raw.ReceivedAt)
	if err != nil {
		return err
	}
	email.ID = uuid.NewString()
	email.ThreadID = raw.ThreadID
	email.RawBody = raw.RawBody
	email.Headers = raw.Headers
	email.ListID = raw.ListID
	email.ListUnsubscribe = raw.ListUnsubscribe

	corrID := uuid.NewString()
	log = logger.WithCorrelation(log, corrID)

	if err := storage.Emails().Save(ctx, email); err != nil {
		return fmt.Errorf("save email: %w", err)
	}
	if err := bus.Publish(ctx, events.EmailFetched{
		UserID:        email.UserID,
		EmailID:       email.ID,
		CorrelationID: corrID,
	}); err != nil {
		return fmt.Errorf("publish EmailFetched: %w", err)
	}

	if cfg.Metrics != nil {
		cfg.Metrics.EmailsFetched.Inc()
	}
	log.Debug("fetcher: stored email", "email_id", email.ID, "gmail_message_id", email.GmailMessageID)
	return nil
}

func fetchWithRetry(ctx context.Context, policy retry.Policy, provider portemail.Provider, params portemail.FetchParams) (*portemail.FetchResult, error) {
	var result *portemail.FetchResult
	err := retry.Do(ctx, policy, func() error {
		var err error
		result, err = provider.Fetch(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func incFetchError(m *metrics.Registry, reason string) {
	if m != nil {
		m.FetchErrors.WithLabelValues(reason).Inc()
	}
}
