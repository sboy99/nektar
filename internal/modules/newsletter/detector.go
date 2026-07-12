package newsletter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/sboy99/nektar/internal/platform/metrics"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"github.com/sboy99/nektar/shared/events"
)

// Config holds detector runtime dependencies beyond ports.
type Config struct {
	ScoreThreshold float64
	Allowlist      []string
	Denylist       []string
	Metrics        *metrics.Registry
}

// Handle processes EmailFetched events and classifies newsletters.
func Handle(
	logger *slog.Logger,
	storage repository.Storage,
	bus porteventbus.EventBus,
	cfg Config,
) porteventbus.Handler {
	classifierCfg := ClassifierConfig{
		ScoreThreshold: cfg.ScoreThreshold,
		Allowlist:      cfg.Allowlist,
		Denylist:       cfg.Denylist,
	}

	return func(ctx context.Context, event porteventbus.Event) error {
		fetched, ok := parseEmailFetched(event)
		if !ok {
			logger.Error("newsletter: unexpected event payload", "event", event.Name())
			return nil
		}

		email, err := storage.Emails().FindByID(ctx, fetched.EmailID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				logger.Warn("newsletter: email not found", "email_id", fetched.EmailID)
				return nil
			}
			return fmt.Errorf("newsletter: find email: %w", err)
		}

		// Idempotent: already classified.
		if email.Stage == domain.StageDetected || email.Stage == domain.StageRejected {
			logger.Debug("newsletter: already processed", "email_id", email.ID, "stage", email.Stage)
			return nil
		}

		result := Classify(email, classifierCfg)
		email.IsNewsletter = result.IsNewsletter
		email.NewsletterScore = result.Score
		if result.ListID != "" {
			email.ListID = result.ListID
		}
		if email.ListUnsubscribe == "" {
			if unsub := resolveListUnsubscribe(email, email.Headers); unsub != "" {
				email.ListUnsubscribe = unsub
			}
		}

		if result.IsNewsletter {
			email.Stage = domain.StageDetected
		} else {
			email.Stage = domain.StageRejected
		}

		if err := storage.Emails().Save(ctx, email); err != nil {
			return fmt.Errorf("newsletter: save email: %w", err)
		}

		if result.IsNewsletter {
			if cfg.Metrics != nil {
				cfg.Metrics.NewslettersDetected.Inc()
			}
			if err := bus.Publish(ctx, events.EmailDetected{
				UserID:  email.UserID,
				EmailID: email.ID,
			}); err != nil {
				return fmt.Errorf("newsletter: publish EmailDetected: %w", err)
			}
			logger.Info("newsletter: detected",
				"email_id", email.ID,
				"score", result.Score,
				"reason", result.Reason,
			)
			return nil
		}

		if cfg.Metrics != nil {
			cfg.Metrics.EmailsRejected.Inc()
		}
		logger.Info("newsletter: rejected",
			"email_id", email.ID,
			"score", result.Score,
			"reason", result.Reason,
		)
		return nil
	}
}

func parseEmailFetched(event porteventbus.Event) (events.EmailFetched, bool) {
	if e, ok := event.(events.EmailFetched); ok {
		return e, true
	}
	switch p := event.Payload().(type) {
	case events.EmailFetched:
		return p, true
	case map[string]any:
		userID, _ := p["UserID"].(string)
		emailID, _ := p["EmailID"].(string)
		if emailID == "" {
			return events.EmailFetched{}, false
		}
		return events.EmailFetched{UserID: userID, EmailID: emailID}, true
	default:
		return events.EmailFetched{}, false
	}
}
