package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sboy99/nektar/internal/bootstrap"
	"github.com/sboy99/nektar/internal/platform/config"
	"github.com/sboy99/nektar/internal/platform/logger"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Logging.Level)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.Build(ctx, cfg, log)
	if err != nil {
		log.Error("failed to bootstrap application", "error", err)
		os.Exit(1)
	}

	log.Info("nektar started",
		"eventbus", cfg.EventBus.Provider,
		"storage", cfg.Storage.Provider,
		"cache", cfg.Cache.Provider,
	)

	if err := app.Run(ctx); err != nil {
		log.Error("application stopped with error", "error", err)
		os.Exit(1)
	}

	log.Info("nektar stopped")
}
