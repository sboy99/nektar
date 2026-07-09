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
│   ├── bootstrap/           # Dependency wiring
│   └── platform/            # Config, logger, metrics, scheduler
├── shared/
│   ├── domain/              # Core domain types (per-entity files)
│   └── events/              # Domain events (user-scoped)
├── configs/                 # YAML configuration
└── deployments/             # Docker Compose for local infra
```

## Prerequisites

- Go 1.23+
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
export NEKTAR_GEMINI_API_KEY=your-key
```

## Endpoints

| Path       | Description          |
|------------|----------------------|
| `/health`  | Health check         |
| `/metrics` | Prometheus metrics   |

## Status

| Phase | Status | Description |
|-------|--------|-------------|
| Foundation | Complete | Architecture, ports, bootstrap, adapters (skeleton) |
| Phase 0 | Complete | Domain model, validation, repository ports, split LLM ports |
| Phase 1+ | Pending | Persistence, pipeline modules, production adapters |

Business logic in modules is stubbed. PostgreSQL adapter methods return `ErrNotImplemented` until Phase 1.

## License

MIT
