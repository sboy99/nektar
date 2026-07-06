package fetcher

import (
	"context"
	"log/slog"

	portemail "github.com/sboy99/nektar/internal/ports/email"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/internal/ports/repository"
)

// Handle returns a scheduler job function that fetches emails.
func Handle(
	logger *slog.Logger,
	email portemail.Provider,
	storage repository.Storage,
	bus porteventbus.EventBus,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		logger.Info("fetcher: scheduled run (stub)")
		_ = email
		_ = storage
		_ = bus
		return nil
	}
}
