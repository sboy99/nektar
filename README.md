# Nektar

**Nektar extracts the nectar (signal) from a sea of information (noise).**

An event-driven newsletter digest pipeline built with Go using Hexagonal Architecture (Ports & Adapters).

## Architecture

```
Scheduler → Fetch Pipeline → Event Bus → [Extractor, Embedding, Clustering, Digest, Publisher]
```

Business modules depend only on **ports** (interfaces). Infrastructure is swappable via configuration:

| Port       | Default Provider | Alternatives (planned)        |
|------------|------------------|-------------------------------|
| Event Bus  | Redis Streams    | In-Memory, Kafka, NATS        |
| Storage    | PostgreSQL       | In-Memory, SQLite             |
| Cache      | Redis            | In-Memory                     |
| LLM        | Gemini           | OpenAI, Claude, Ollama        |
| Email      | Gmail            | Outlook, IMAP                 |
| Publisher  | Discord          | Slack, Telegram, Email, RSS   |

## Project Layout

```
nektar/
├── cmd/nektar/              # Application entrypoint
├── internal/
│   ├── modules/             # Business capabilities (handler stubs)
│   ├── ports/               # Interface contracts
│   ├── adapters/            # Infrastructure implementations
│   ├── bootstrap/           # Dependency wiring
│   └── platform/            # Config, logger, metrics, scheduler
├── shared/
│   ├── domain/              # Core domain types
│   └── events/              # Domain events
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

This is the **foundation scaffold**. Business logic in modules is stubbed. Adapter methods that require domain logic return `ErrNotImplemented`.

## License

MIT
