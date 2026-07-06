package embedding

import (
	"context"
	"log/slog"

	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
)

// Handle processes ArticleCreated events.
func Handle(logger *slog.Logger) porteventbus.Handler {
	return func(ctx context.Context, event porteventbus.Event) error {
		logger.Info("embedding: received event (stub)", "event", event.Name())
		return nil
	}
}
