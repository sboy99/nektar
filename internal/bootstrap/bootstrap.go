package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sboy99/nektar/internal/adapters/discord"
	"github.com/sboy99/nektar/internal/adapters/gemini"
	"github.com/sboy99/nektar/internal/adapters/gmail"
	"github.com/sboy99/nektar/internal/adapters/inmemory"
	"github.com/sboy99/nektar/internal/adapters/postgres"
	redisadapter "github.com/sboy99/nektar/internal/adapters/redis"
	"github.com/sboy99/nektar/internal/modules/clustering"
	"github.com/sboy99/nektar/internal/modules/digest"
	"github.com/sboy99/nektar/internal/modules/embedding"
	"github.com/sboy99/nektar/internal/modules/extractor"
	"github.com/sboy99/nektar/internal/modules/fetcher"
	"github.com/sboy99/nektar/internal/modules/newsletter"
	modpublisher "github.com/sboy99/nektar/internal/modules/publisher"
	"github.com/sboy99/nektar/internal/platform/config"
	"github.com/sboy99/nektar/internal/platform/metrics"
	"github.com/sboy99/nektar/internal/platform/scheduler"
	portcache "github.com/sboy99/nektar/internal/ports/cache"
	portemail "github.com/sboy99/nektar/internal/ports/email"
	portembedding "github.com/sboy99/nektar/internal/ports/embedding"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	portpublisher "github.com/sboy99/nektar/internal/ports/publisher"
	"github.com/sboy99/nektar/internal/ports/repository"
	portsummary "github.com/sboy99/nektar/internal/ports/summary"
	"github.com/sboy99/nektar/shared/events"
)

// App holds all wired dependencies.
type App struct {
	Config     *config.Config
	Logger     *slog.Logger
	Metrics    *metrics.Registry
	Storage    repository.Storage
	EventBus   porteventbus.EventBus
	Cache      portcache.Cache
	Embedder   portembedding.Provider
	Summarizer portsummary.Provider
	Email      portemail.Provider
	Publisher  portpublisher.Publisher
	Scheduler  *scheduler.Scheduler
}

// Build wires all dependencies from configuration.
func Build(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*App, error) {
	app := &App{
		Config:    cfg,
		Logger:    logger,
		Metrics:   metrics.NewRegistry(),
		Scheduler: scheduler.New(logger),
	}

	var err error

	app.Storage, err = buildStorage(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build storage: %w", err)
	}

	app.EventBus, err = buildEventBus(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("build event bus: %w", err)
	}

	app.Cache, err = buildCache(cfg)
	if err != nil {
		return nil, fmt.Errorf("build cache: %w", err)
	}

	geminiProvider, err := buildGemini(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build gemini: %w", err)
	}
	app.Embedder = geminiProvider
	app.Summarizer = geminiProvider

	app.Email, err = buildEmail(cfg)
	if err != nil {
		return nil, fmt.Errorf("build email: %w", err)
	}

	app.Publisher, err = buildPublisher(cfg)
	if err != nil {
		return nil, fmt.Errorf("build publisher: %w", err)
	}

	registerHandlers(ctx, app)
	registerSchedulerJobs(app)

	return app, nil
}

func buildStorage(ctx context.Context, cfg *config.Config) (repository.Storage, error) {
	switch cfg.Storage.Provider {
	case "postgres":
		return postgres.NewStorage(ctx, cfg.Postgres.DSN)
	case "inmemory":
		return inmemory.NewStorage(), nil
	default:
		return nil, fmt.Errorf("unknown storage provider: %s", cfg.Storage.Provider)
	}
}

func buildEventBus(cfg *config.Config, logger *slog.Logger) (porteventbus.EventBus, error) {
	switch cfg.EventBus.Provider {
	case "redis":
		client := redisadapter.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		return redisadapter.NewBus(redisadapter.BusConfig{
			Client:       client,
			StreamPrefix: cfg.Redis.StreamPrefix,
			GroupName:    cfg.Redis.GroupName,
			ConsumerName: cfg.Redis.ConsumerName,
			Logger:       logger,
		}), nil
	case "inmemory":
		return inmemory.NewBus(), nil
	default:
		return nil, fmt.Errorf("unknown eventbus provider: %s", cfg.EventBus.Provider)
	}
}

func buildCache(cfg *config.Config) (portcache.Cache, error) {
	switch cfg.Cache.Provider {
	case "redis":
		client := redisadapter.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		return redisadapter.NewCache(client), nil
	case "inmemory":
		return inmemory.NewCache(), nil
	default:
		return nil, fmt.Errorf("unknown cache provider: %s", cfg.Cache.Provider)
	}
}

func buildGemini(ctx context.Context, cfg *config.Config) (*gemini.Provider, error) {
	switch cfg.LLM.Provider {
	case "gemini":
		return gemini.NewProvider(ctx, gemini.Config{
			APIKey:     cfg.Gemini.APIKey,
			Model:      cfg.Gemini.Model,
			EmbedModel: cfg.Gemini.EmbedModel,
			Dimensions: cfg.Gemini.EmbedDimensions,
		})
	default:
		return nil, fmt.Errorf("unknown llm provider: %s", cfg.LLM.Provider)
	}
}

func buildEmail(cfg *config.Config) (portemail.Provider, error) {
	switch cfg.Email.Provider {
	case "gmail":
		return gmail.NewProvider(gmail.Config{
			ClientID:     cfg.Gmail.ClientID,
			ClientSecret: cfg.Gmail.ClientSecret,
			DefaultQuery: cfg.Gmail.DefaultQuery,
		}), nil
	default:
		return nil, fmt.Errorf("unknown email provider: %s", cfg.Email.Provider)
	}
}

func buildPublisher(cfg *config.Config) (portpublisher.Publisher, error) {
	switch cfg.Publisher.Provider {
	case "discord":
		return discord.NewPublisher(cfg.Discord.WebhookURL), nil
	default:
		return nil, fmt.Errorf("unknown publisher provider: %s", cfg.Publisher.Provider)
	}
}

func registerHandlers(ctx context.Context, app *App) {
	log := app.Logger

	_ = app.EventBus.Subscribe(ctx, events.TopicEmailFetched, newsletter.Handle(log, app.Storage, app.EventBus, newsletter.Config{
		ScoreThreshold: app.Config.Newsletter.ScoreThreshold,
		Allowlist:      app.Config.Newsletter.Allowlist,
		Denylist:       app.Config.Newsletter.Denylist,
		Metrics:        app.Metrics,
	}))
	_ = app.EventBus.Subscribe(ctx, events.TopicEmailDetected, extractor.Handle(log, app.Storage, app.EventBus, extractor.Config{
		MinArticleChars: app.Config.Extraction.MinArticleChars,
		WordsPerMinute:  app.Config.Extraction.WordsPerMinute,
		Metrics:         app.Metrics,
	}))
	_ = app.EventBus.Subscribe(ctx, events.TopicArticleCreated, embedding.Handle(
		log, app.Storage, app.EventBus, app.Embedder, app.Cache, embedding.Config{
			Provider:        app.Config.LLM.Provider,
			Model:           app.Config.Gemini.EmbedModel,
			Dimensions:      app.Config.Gemini.EmbedDimensions,
			CacheTTL:        app.Config.Embedding.CacheTTL,
			CostPer1MTokens: app.Config.Gemini.EmbedCostPer1MTokens,
			Metrics:         app.Metrics,
		},
	))
	_ = app.EventBus.Subscribe(ctx, events.TopicEmbeddingCreated, clustering.Handle(log))
	_ = app.EventBus.Subscribe(ctx, events.TopicClusterUpdated, digest.Handle(log))
	_ = app.EventBus.Subscribe(ctx, events.TopicDigestReady, modpublisher.Handle(log, app.Publisher))
}

func registerSchedulerJobs(app *App) {
	app.Scheduler.Register(scheduler.Job{
		Name:     "fetcher",
		Interval: app.Config.Scheduler.FetchInterval,
		Fn: fetcher.Handle(app.Logger, app.Email, app.Storage, app.EventBus, fetcher.Config{
			RefreshTokens: app.Config.Gmail.RefreshTokens,
			DefaultQuery:  app.Config.Gmail.DefaultQuery,
			Metrics:       app.Metrics,
		}),
	})
}

// Run starts the scheduler and metrics HTTP server.
func (a *App) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    a.Config.Server.Addr,
		Handler: mux,
	}

	go func() {
		a.Logger.Info("starting http server", "addr", a.Config.Server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.Logger.Error("http server failed", "error", err)
		}
	}()

	a.Scheduler.Start(ctx)

	<-ctx.Done()

	a.Scheduler.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)

	return a.Close()
}

// Close releases all resources.
func (a *App) Close() error {
	if a.EventBus != nil {
		_ = a.EventBus.Close()
	}
	if a.Storage != nil {
		_ = a.Storage.Close()
	}
	return nil
}
