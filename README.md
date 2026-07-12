# Nektar

<p align="center">
  <img src="docs/nektar-banner.png" alt="Nektar — Your daily dose of tech intelligence" width="680" />
</p>

**Your daily dose of tech intelligence.**

Nektar delivers the most important tech updates straight to your Discord — curated, concise, and AI-powered. It extracts the nectar (signal) from a sea of information (noise).

An event-driven technical intelligence platform built with Go using Hexagonal Architecture (Ports & Adapters).

Full roadmap: [TASKS.md](TASKS.md)

**Install, setup & user guide:** [docs/GUIDE.md](docs/GUIDE.md)

## Pipeline

```
Scheduler
    │
    ▼
Fetch Emails
    │
    ▼
Newsletter Detection
    │
    ▼
Content Extraction
    │
    ▼
Normalization
    │
    ▼
Article Splitter
    │
    ▼
Embedding Generation
    │
    ▼
Topic Clustering
    │
    ▼
Digest Builder
    │
    ▼
Publisher
```

Business modules depend only on **ports** (interfaces). Infrastructure is swappable via configuration:

| Port       | Default Provider | Alternatives (planned)        |
|------------|------------------|-------------------------------|
| Event Bus  | Redis Streams    | In-Memory, Kafka, NATS        |
| Storage    | PostgreSQL       | In-Memory, SQLite             |
| Cache      | Redis            | In-Memory                     |
| Embedding  | Gemini           | OpenAI, Claude, Ollama        |
| Summary    | Gemini           | OpenAI, Claude, Ollama        |
| Email      | Gmail            | Outlook, IMAP                 |
| Publisher  | Discord          | Slack, Telegram, Email, RSS   |

## Domain Model

Multi-user domain model in `shared/domain/`:

| Aggregate | Entity | Description |
|-----------|--------|-------------|
| User | `User`, `GmailSync` | Account identity and Gmail sync cursor |
| Email | `Email` | Fetched message with newsletter detection fields |
| Article | `Article` | Extracted and normalized content |
| Cluster | `Cluster` | Semantic grouping of articles |
| Topic | `Topic` | Human-facing label for a cluster |
| Digest | `Digest` | Curated summary ready to publish |

Supporting entities: `Embedding`, `LLMRequest`, `PipelineStatus`

## Persistence

PostgreSQL is the production storage adapter. On connect, `postgres.NewStorage` runs embedded migrations via [golang-migrate](https://github.com/golang-migrate/migrate).

Schema lives in `internal/adapters/postgres/migrations/`:

| Table | Purpose |
|-------|---------|
| `users` | Account identity |
| `gmail_sync` | Incremental Gmail sync cursor |
| `emails` | Fetched messages |
| `articles` | Extracted content |
| `clusters` / `cluster_articles` | Semantic groupings |
| `embeddings` | Article vectors (`REAL[]`) |
| `digests` | Curated summaries |
| `llm_requests` | Cost / latency tracking |

Repository ports are fully implemented for both PostgreSQL and in-memory storage.

## Project Layout

```
nektar/
├── cmd/nektar/              # Application entrypoint
├── cmd/nektar-gmail-auth/   # OAuth helper to obtain Gmail refresh tokens
├── internal/
│   ├── modules/             # Business capabilities (fetcher → publisher, jobs)
│   ├── ports/               # Interface contracts
│   │   ├── embedding/       # EmbeddingProvider
│   │   ├── summary/         # SummaryProvider
│   │   ├── repository/      # Per-aggregate repositories
│   │   └── ...
│   ├── adapters/            # Infrastructure implementations
│   │   └── postgres/        # Storage + embedded SQL migrations
│   ├── bootstrap/           # Dependency wiring
│   └── platform/            # Config, logger, metrics, scheduler
├── shared/
│   ├── domain/              # Core domain types (per-entity files)
│   └── events/              # Domain events (user-scoped)
├── configs/                 # YAML configuration
└── deployments/             # Docker Compose for local infra
```

## Prerequisites

- Go 1.25+
- Docker & Docker Compose (for Redis + PostgreSQL)

## Quick Start

```bash
# Start infrastructure
make infra-up

# Install dependencies
make deps

# Build
make build

# Run (requires Gmail/Gemini/Discord credentials in config or env)
make run
```

### Local Development (no external services)

Edit `configs/config.yaml` to use in-memory providers:

```yaml
eventbus:
  provider: inmemory
storage:
  provider: inmemory
cache:
  provider: inmemory
```

Note: Gemini still requires a valid API key to bootstrap. Gmail OAuth app credentials are needed for live fetch; refresh tokens are resolved per user at runtime (see Gmail setup below).

### Gmail OAuth Setup

1. Create an OAuth 2.0 Client ID (Desktop or Web) in Google Cloud Console with redirect URI `http://localhost:8085/oauth2/callback`.
2. Enable the Gmail API for the project.
3. Obtain a refresh token:

```bash
go run ./cmd/nektar-gmail-auth \
  -client-id="$NEKTAR_GMAIL_CLIENT_ID" \
  -client-secret="$NEKTAR_GMAIL_CLIENT_SECRET" \
  -ref=alice
```

4. Put the printed snippet into `configs/config.yaml` under `gmail.refresh_tokens`.
5. Seed a user and sync cursor (example SQL):

```sql
INSERT INTO users (id, email, name, created_at, updated_at)
VALUES ('user-1', 'you@example.com', 'You', now(), now());

INSERT INTO gmail_sync (user_id, history_id, last_synced_at, query, refresh_token_ref)
VALUES ('user-1', '', now(), 'newer_than:7d', 'alice');
```

`refresh_token_ref` must match a key in `gmail.refresh_tokens`. Leave `history_id` empty for the first bootstrap list sync; subsequent runs use the History API.

### Newsletter Detection

After fetch, emails are classified heuristically (no LLM). Confirmed newsletters emit `email.detected` for extraction; others are stored as `rejected`.

Signals: List-ID, List-Unsubscribe, Precedence/bulk headers, campaign/ESP headers, spam flags, plus optional sender allow/deny lists.

```yaml
newsletter:
  score_threshold: 0.5
  allowlist: []   # e.g. "news@tldr.tech" or "@substack.com"
  denylist: []
```

### Content Extraction

On `email.detected`, the extractor parses MIME/HTML/plain bodies, strips nav/footer/ads/unsubscribe chrome, converts to Markdown, and heuristically splits multi-story newsletters (headings / `<hr>`). Each article is persisted and emits `article.created`.

```yaml
extraction:
  min_article_chars: 100
  words_per_minute: 200
```

### Embedding Generation

On `article.created`, the embedding module embeds article plain text (Markdown fallback) via the EmbeddingProvider (Gemini by default), caches by content hash, persists the vector + an `LLMRequest` cost record, and emits `embedding.created`.

```yaml
gemini:
  embed_model: gemini-embedding-001
  embed_dimensions: 768
  embed_cost_per_1m_tokens: 0.15

embedding:
  cache_ttl: 720h
```

### Scheduled Jobs

Jobs run immediately on startup, then on a fixed interval. Configure under `scheduler:`:

| Job | Config key | Default | Behavior |
|-----|------------|---------|----------|
| `fetch_gmail` | `fetch_interval` | `15m` | Fetch new emails for all Gmail sync users |
| `retry_failures` | `retry_interval` | `5m` | Replay DLQ messages onto main event streams |
| `publish_digest` | `publish_interval` | `1h` | Publish ready unpublished digests (catch-up) |
| `cleanup_cache` | `cache_cleanup_interval` | `24h` | Purge expired in-memory cache entries |
| `cleanup_old_events` | `events_cleanup_interval` | `24h` | Trim old stream entries and delete emails older than `event_retention` |
| `refresh_oauth` | `oauth_refresh_interval` | `12h` | Probe/refresh Gmail OAuth tokens |

```yaml
scheduler:
  fetch_interval: 15m
  retry_interval: 5m
  publish_interval: 1h
  cache_cleanup_interval: 24h
  events_cleanup_interval: 24h
  oauth_refresh_interval: 12h
  dlq_replay_limit: 100
  event_retention: 168h
```

### Environment Overrides / Secrets

Secrets load from `.env` (see `.env.example`). Any config value can also be set via `NEKTAR_*` env vars (OS env wins over `.env`):

```bash
cp .env.example .env
# edit .env — Gmail, Gemini, Discord, optional DSN

export NEKTAR_EVENTBUS_PROVIDER=inmemory
export NEKTAR_STORAGE_PROVIDER=postgres
export NEKTAR_POSTGRES_DSN='postgres://nektar:nektar@localhost:5432/nektar?sslmode=disable'
```

### Integration Tests

Postgres and Redis adapter tests skip unless env vars are set. CI runs them against service containers; locally:

```bash
make infra-up
make test-integration
```

Or explicitly:

```bash
NEKTAR_POSTGRES_DSN='postgres://nektar:nektar@localhost:5432/nektar?sslmode=disable' \
NEKTAR_REDIS_ADDR='localhost:6379' \
  go test ./internal/adapters/postgres/... ./internal/adapters/redis/... -count=1 -v
```

Unit and in-memory E2E pipeline tests run with plain `make test` / `go test ./...` (no infra required).

## Endpoints

| Path       | Description                                      |
|------------|--------------------------------------------------|
| `/health`  | Liveness probe (process up)                      |
| `/ready`   | Readiness probe (Postgres + Redis/cache ping)    |
| `/metrics` | Prometheus metrics                               |

Structured JSON logs include `correlation_id` across the pipeline. Configure log level via `logging.level` (`debug|info|warn|error`).

## Status

| Phase | Status | Description |
|-------|--------|-------------|
| Foundation | Complete | Architecture, ports, bootstrap, adapters |
| Phase 0 | Complete | Domain model, validation, repository ports, split LLM ports |
| Phase 1 | Complete | PostgreSQL schema, migrations, repository implementations |
| Phase 2 | Complete | Multi-user Gmail fetch, History sync, EmailFetched publishing |
| Phase 3 | Complete | Heuristic newsletter detection, EmailDetected, allow/deny lists |
| Phase 4 | Complete | Extraction pipeline, article split, ArticleCreated |
| Phase 5 | Complete | Embedding generation, cache, cost tracking, EmbeddingCreated |
| Phase 6 | Complete | Topic clustering, ClusterUpdated |
| Phase 7 | Complete | Digest builder, prompts, DigestReady |
| Phase 8 | Complete | Discord publisher, webhook retry, publish metrics |
| Phase 9 | Complete | Scheduler jobs (fetch, retry, publish, cleanup, OAuth) |
| Phase 10 | Complete | Metrics, correlation IDs, retry policy, readiness, graceful shutdown |
| Phase 11 | Complete | Unit, adapter integration (Postgres/Redis), E2E pipeline |

## License

MIT
