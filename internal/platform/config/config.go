package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	EventBus  ProviderConfig `mapstructure:"eventbus"`
	Storage   ProviderConfig `mapstructure:"storage"`
	Cache     ProviderConfig `mapstructure:"cache"`
	LLM       ProviderConfig `mapstructure:"llm"`
	Email     ProviderConfig `mapstructure:"email"`
	Publisher ProviderConfig `mapstructure:"publisher"`

	Redis      RedisConfig      `mapstructure:"redis"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Gmail      GmailConfig      `mapstructure:"gmail"`
	Gemini     GeminiConfig     `mapstructure:"gemini"`
	Discord    DiscordConfig    `mapstructure:"discord"`
	Newsletter NewsletterConfig `mapstructure:"newsletter"`
	Extraction ExtractionConfig `mapstructure:"extraction"`
	Embedding  EmbeddingConfig  `mapstructure:"embedding"`
	Clustering ClusteringConfig `mapstructure:"clustering"`
	Digest     DigestConfig     `mapstructure:"digest"`
	Scheduler  SchedulerConfig  `mapstructure:"scheduler"`
	Server     ServerConfig     `mapstructure:"server"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Retry      RetryConfig      `mapstructure:"retry"`
}

// ProviderConfig selects an infrastructure implementation.
type ProviderConfig struct {
	Provider string `mapstructure:"provider"`
}

// RedisConfig configures Redis connections.
type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	StreamPrefix string `mapstructure:"stream_prefix"`
	GroupName    string `mapstructure:"group_name"`
	ConsumerName string `mapstructure:"consumer_name"`
}

// PostgresConfig configures PostgreSQL connections.
type PostgresConfig struct {
	DSN string `mapstructure:"dsn"`
}

// GmailConfig configures Gmail API access.
type GmailConfig struct {
	ClientID      string            `mapstructure:"client_id"`
	ClientSecret  string            `mapstructure:"client_secret"`
	DefaultQuery  string            `mapstructure:"default_query"`
	RefreshTokens map[string]string `mapstructure:"refresh_tokens"`
}

// GeminiConfig configures the Gemini LLM provider.
type GeminiConfig struct {
	APIKey               string  `mapstructure:"api_key"`
	Model                string  `mapstructure:"model"`
	EmbedModel           string  `mapstructure:"embed_model"`
	EmbedDimensions      int     `mapstructure:"embed_dimensions"`
	EmbedCostPer1MTokens float64 `mapstructure:"embed_cost_per_1m_tokens"`
}

// EmbeddingConfig configures the embedding pipeline.
type EmbeddingConfig struct {
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

// ClusteringConfig configures topic clustering.
type ClusteringConfig struct {
	SimilarityThreshold float64 `mapstructure:"similarity_threshold"`
	MergeThreshold      float64 `mapstructure:"merge_threshold"`
}

// DigestConfig configures the digest builder pipeline.
type DigestConfig struct {
	PromptsDir            string        `mapstructure:"prompts_dir"`
	Lookback              time.Duration `mapstructure:"lookback"`
	MinArticles           int           `mapstructure:"min_articles"`
	MinClusters           int           `mapstructure:"min_clusters"`
	MinClusterSize        int           `mapstructure:"min_cluster_size"`
	MaxClusters           int           `mapstructure:"max_clusters"`
	MaxArticlesPerCluster int           `mapstructure:"max_articles_per_cluster"`
	WordsPerMinute        int           `mapstructure:"words_per_minute"`
	CostPer1MTokens       float64       `mapstructure:"cost_per_1m_tokens"`
}

// DiscordConfig configures Discord webhook publishing.
type DiscordConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

// NewsletterConfig configures newsletter detection heuristics.
type NewsletterConfig struct {
	ScoreThreshold float64  `mapstructure:"score_threshold"`
	Allowlist      []string `mapstructure:"allowlist"`
	Denylist       []string `mapstructure:"denylist"`
}

// ExtractionConfig configures the content extraction pipeline.
type ExtractionConfig struct {
	MinArticleChars int `mapstructure:"min_article_chars"`
	WordsPerMinute  int `mapstructure:"words_per_minute"`
}

// SchedulerConfig configures the job scheduler.
type SchedulerConfig struct {
	FetchInterval         time.Duration `mapstructure:"fetch_interval"`
	RetryInterval         time.Duration `mapstructure:"retry_interval"`
	PublishInterval       time.Duration `mapstructure:"publish_interval"`
	CacheCleanupInterval  time.Duration `mapstructure:"cache_cleanup_interval"`
	EventsCleanupInterval time.Duration `mapstructure:"events_cleanup_interval"`
	OAuthRefreshInterval  time.Duration `mapstructure:"oauth_refresh_interval"`
	DLQReplayLimit        int           `mapstructure:"dlq_replay_limit"`
	EventRetention        time.Duration `mapstructure:"event_retention"`
}

// ServerConfig configures the HTTP server for metrics/health.
type ServerConfig struct {
	Addr            string        `mapstructure:"addr"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// LoggingConfig configures structured logging.
type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

// RetryConfig configures shared retry policies.
type RetryConfig struct {
	EventBusMaxAttempts int           `mapstructure:"eventbus_max_attempts"`
	EventBusBaseBackoff time.Duration `mapstructure:"eventbus_base_backoff"`
	FetchMaxAttempts    int           `mapstructure:"fetch_max_attempts"`
	FetchBaseBackoff    time.Duration `mapstructure:"fetch_base_backoff"`
}

// Load reads configuration from the given path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("NEKTAR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("eventbus.provider", "inmemory")
	v.SetDefault("storage.provider", "inmemory")
	v.SetDefault("cache.provider", "inmemory")
	v.SetDefault("llm.provider", "gemini")
	v.SetDefault("email.provider", "gmail")
	v.SetDefault("publisher.provider", "discord")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.stream_prefix", "nektar")
	v.SetDefault("redis.group_name", "nektar-workers")
	v.SetDefault("redis.consumer_name", "nektar-1")

	v.SetDefault("postgres.dsn", "postgres://nektar:nektar@localhost:5432/nektar?sslmode=disable")

	v.SetDefault("gmail.default_query", "newer_than:7d")

	v.SetDefault("gemini.model", "gemini-2.0-flash")
	v.SetDefault("gemini.embed_model", "gemini-embedding-001")
	v.SetDefault("gemini.embed_dimensions", 768)
	v.SetDefault("gemini.embed_cost_per_1m_tokens", 0.15)

	v.SetDefault("newsletter.score_threshold", 0.5)

	v.SetDefault("extraction.min_article_chars", 100)
	v.SetDefault("extraction.words_per_minute", 200)

	v.SetDefault("embedding.cache_ttl", "720h")

	v.SetDefault("clustering.similarity_threshold", 0.75)
	v.SetDefault("clustering.merge_threshold", 0.90)

	v.SetDefault("digest.prompts_dir", "prompts")
	v.SetDefault("digest.lookback", "24h")
	v.SetDefault("digest.min_articles", 3)
	v.SetDefault("digest.min_clusters", 1)
	v.SetDefault("digest.min_cluster_size", 1)
	v.SetDefault("digest.max_clusters", 10)
	v.SetDefault("digest.max_articles_per_cluster", 5)
	v.SetDefault("digest.words_per_minute", 200)
	v.SetDefault("digest.cost_per_1m_tokens", 0.10)

	v.SetDefault("scheduler.fetch_interval", "15m")
	v.SetDefault("scheduler.retry_interval", "5m")
	v.SetDefault("scheduler.publish_interval", "1h")
	v.SetDefault("scheduler.cache_cleanup_interval", "24h")
	v.SetDefault("scheduler.events_cleanup_interval", "24h")
	v.SetDefault("scheduler.oauth_refresh_interval", "12h")
	v.SetDefault("scheduler.dlq_replay_limit", 100)
	v.SetDefault("scheduler.event_retention", "168h")
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.shutdown_timeout", "10s")
	v.SetDefault("logging.level", "info")
	v.SetDefault("retry.eventbus_max_attempts", 3)
	v.SetDefault("retry.eventbus_base_backoff", "100ms")
	v.SetDefault("retry.fetch_max_attempts", 3)
	v.SetDefault("retry.fetch_base_backoff", "500ms")
}
