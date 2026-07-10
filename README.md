# Nektar

**Nektar extracts the nectar (signal) from a sea of information (noise).**

An event-driven technical intelligence platform built with Go using Hexagonal Architecture (Ports & Adapters).

Full roadmap: [TASKS.md](TASKS.md)

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
├── internal/
│   ├── modules/             # Business capabilities (handler stubs)
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

Note: Gmail and Gemini adapters still require valid credentials to bootstrap. Set empty credentials only if you plan to stub those adapters for local dev.

### Environment Overrides

Any config value can be overridden via environment variables with the `NEKTAR_` prefix:

```bash
export NEKTAR_EVENTBUS_PROVIDER=inmemory
export NEKTAR_STORAGE_PROVIDER=postgres
export NEKTAR_POSTGRES_DSN='postgres://nektar:nektar@localhost:5432/nektar?sslmode=disable'
export NEKTAR_GEMINI_API_KEY=your-key
```

### PostgreSQL Integration Tests

Postgres repository tests skip unless a DSN is set:

```bash
make infra-up
NEKTAR_POSTGRES_DSN='postgres://nektar:nektar@localhost:5432/nektar?sslmode=disable' \
  go test ./internal/adapters/postgres/... -count=1 -v
```

## Endpoints

| Path       | Description          |
|------------|----------------------|
| `/health`  | Health check         |
| `/metrics` | Prometheus metrics   |

## Status

| Phase | Status | Description |
|-------|--------|-------------|
| Foundation | Complete | Architecture, ports, bootstrap, adapters |
| Phase 0 | Complete | Domain model, validation, repository ports, split LLM ports |
| Phase 1 | Complete | PostgreSQL schema, migrations, repository implementations |
| Phase 2+ | Pending | Fetch pipeline, detection, extraction, and remaining modules |

Business logic in modules is still stubbed. Persistence is ready for Phase 2 to start writing through the repository ports.

## License

MIT
