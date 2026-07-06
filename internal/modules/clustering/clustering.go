package clustering

import (
	"context"
	"log/slog"

	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
)

// Handle processes EmbeddingCreated events.
func Handle(logger *slog.Logger) porteventbus.Handler {
	return func(ctx context.Context, event porteventbus.Event) error {
		logger.Info("clustering: received event (stub)", "event", event.Name())
		return nil
	}
}
