---
name: Nektar Foundation Scaffold
overview: "Bootstrap the Nektar Go project as a Ports & Adapters (Hexagonal) foundation: repo layout, config-driven wiring, all port interfaces, a working event bus (in-memory + Redis Streams), and skeleton adapters for the selected providers (Redis, PostgreSQL, Gmail, Gemini, Discord) — with no business logic in modules yet."
todos:
  - id: init
    content: Initialize go module (github.com/sboy99/nektar, Go 1.23+), .gitignore, git init, base directory tree under cmd/internal/shared/configs/deployments.
    status: completed
  - id: config
    content: Add platform/config (viper-based) loading configs/config.yaml with provider selection keys (eventbus/storage/cache/llm/email/publisher) plus env overrides.
    status: completed
  - id: shared
    content: Define shared/domain core types (Article, Digest, Email, Embedding, Cluster) and shared/events with the five event types implementing the Event interface.
    status: completed
  - id: ports
    content: "Define all port interfaces under internal/ports: eventbus (EventBus/Event/Handler), llm.Provider, email.Provider, repository (Article/Digest/Email), cache.Cache, publisher.Publisher."
    status: completed
  - id: eventbus-inmemory
    content: Implement internal/adapters/inmemory event bus that synchronously dispatches to subscribers (fast tests) plus in-memory cache/repository stubs.
    status: completed
  - id: eventbus-redis
    content: Implement internal/adapters/redis event bus over Redis Streams (XADD/XREADGROUP/ACK, retry + DLQ) and redis cache adapter.
    status: completed
  - id: adapters-skeleton
    content: Scaffold postgres (pgx repos), gmail (oauth2 client), gemini (genai client), discord (webhook) adapters satisfying their ports; unimplemented methods return ErrNotImplemented.
    status: completed
  - id: platform
    content: "Add platform pieces: slog logger, prometheus metrics registry, scheduler abstraction."
    status: completed
  - id: bootstrap
    content: Add internal/bootstrap provider factory/registry that reads config and constructs each dependency, then wires cmd/nektar/main.go through the LLD bootstrap flow with empty handler registrations.
    status: completed
  - id: modules-stubs
    content: Create modules/{fetcher,extractor,embedding,clustering,digest,publisher} with Handle stubs matching the subscriber signatures (no business logic).
    status: completed
  - id: ops
    content: Add deployments/docker-compose.yaml (redis + postgres), Makefile (build/run/test/lint), and README documenting layout and how to run; verify go build ./... compiles.
    status: completed
isProject: false
---

# Nektar Foundation Scaffold

Greenfield: the repo currently contains only [.cursor/lld.md](.cursor/lld.md). This plan lays the **foundation only** — everything compiles and boots, but business logic inside `modules/` is intentionally left as empty handler stubs.

## Architecture (target layout)

Following the LLD's final Ports & Adapters recommendation ([.cursor/lld.md](.cursor/lld.md) lines 543-575):

```text
nektar/
  cmd/nektar/main.go
  internal/
    modules/           # business capabilities (empty handler stubs for now)
      fetcher/ extractor/ embedding/ clustering/ digest/ publisher/
    ports/             # interfaces (contracts) only
      eventbus/ llm/ email/ repository/ cache/ publisher/
    adapters/          # implementations
      inmemory/ redis/ postgres/ gmail/ gemini/ discord/
    bootstrap/         # config-driven dependency wiring
    platform/          # cross-cutting: logger, config, scheduler, metrics
  shared/
    domain/ events/
  configs/config.yaml
  deployments/docker-compose.yaml
```

Dependency rule (LLD lines 521-539): `modules` and `shared/domain` import only `ports`, never `adapters`.

```mermaid
flowchart TD
  Domain --> Ports
  Modules --> Ports
  Bootstrap --> Ports
  Bootstrap --> Adapters
  Adapters --> Ports
```

## Tech choices (reasonable defaults, adjustable)

- Module path `github.com/sagarbera/nektar`, Go 1.23+.
- Config: `spf13/viper` (YAML + env override), config-driven provider selection per LLD lines 410-442.
- Event bus Redis + cache Redis: `redis/go-redis/v9`.
- Storage: `jackc/pgx/v5` for PostgreSQL.
- Logger: stdlib `log/slog`. Metrics: `prometheus/client_golang`.
- Gmail: `google.golang.org/api/gmail/v1` + `golang.org/x/oauth2`. Gemini: `google.golang.org/genai`. Discord: webhook via stdlib `net/http` (no heavy dep).

## Scope boundaries

- **In scope:** repo layout, `go.mod`, config, all port interfaces, event bus (in-memory + Redis Streams w/ consumer groups/ACK/DLQ), provider factory/registry, bootstrap wiring, skeleton adapters that satisfy interfaces (real client setup where trivial, `ErrNotImplemented` for methods needing business logic), logger/metrics/scheduler platform pieces, docker-compose (redis + postgres), Makefile, README.
- **Out of scope (deferred):** actual fetch/extract/embed/cluster/digest logic, Kafka/NATS/Outlook/IMAP/OpenAI/Claude/Ollama/Slack/Telegram/SQLite adapters, DB migrations content beyond initial schema, auth/token flows beyond config plumbing.

## Key interfaces to define (from LLD)

- `EventBus` (Publish/Subscribe/Close), `Event` (Name/Payload), `Handler` — lines 168-201.
- `llm.Provider` (Embed/Summarize) — lines 287-299.
- `email.Provider` (Fetch) — lines 315-333.
- `repository`: `ArticleRepository`, `DigestRepository`, `EmailRepository` — lines 337-359.
- `cache.Cache` (Get/Set/Delete) — lines 363-385.
- `publisher.Publisher` — lines 388-406.
- `shared/events`: `EmailFetched`, `ArticleCreated`, `EmbeddingCreated`, `ClusterUpdated`, `DigestReady` — lines 205-211, 488-515.

## Bootstrap flow (LLD lines 446-517)

`main.go` -> load config -> build storage -> event bus -> cache -> llm -> publishers -> register handlers (stubs) -> start workers/scheduler.