package publisher

import (
	"context"
	"log/slog"

	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	portpublisher "github.com/sboy99/nektar/internal/ports/publisher"
)

// Handle processes DigestReady events.
func Handle(logger *slog.Logger, pub portpublisher.Publisher) porteventbus.Handler {
	return func(ctx context.Context, event porteventbus.Event) error {
		logger.Info("publisher: received event (stub)", "event", event.Name())
		_ = pub
		return nil
	}
}
