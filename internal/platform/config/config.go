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
	Scheduler  SchedulerConfig  `mapstructure:"scheduler"`
	Server     ServerConfig     `mapstructure:"server"`
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
	APIKey     string `mapstructure:"api_key"`
	Model      string `mapstructure:"model"`
	EmbedModel string `mapstructure:"embed_model"`
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

// SchedulerConfig configures the job scheduler.
type SchedulerConfig struct {
	FetchInterval time.Duration `mapstructure:"fetch_interval"`
}

// ServerConfig configures the HTTP server for metrics/health.
type ServerConfig struct {
	Addr string `mapstructure:"addr"`
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
	v.SetDefault("gemini.embed_model", "text-embedding-004")

	v.SetDefault("newsletter.score_threshold", 0.5)

	v.SetDefault("scheduler.fetch_interval", "15m")
	v.SetDefault("server.addr", ":8080")
}
