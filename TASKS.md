# Nektar — Project Tasks

Track remaining work to take Nektar from the current **foundation scaffold** to a complete, production-ready newsletter digest pipeline.

Reference: [.cursor/lld.md](.cursor/lld.md)

---

## Current Status

The foundation is in place: Hexagonal layout, all port interfaces, config-driven bootstrap, in-memory + Redis event bus, skeleton adapters, and stubbed module handlers. `go build ./...` and `go test ./...` pass.

What remains is **business logic**, **real adapter implementations**, **database schema**, **auth flows**, **tests**, and **production hardening**.

---

## Completed

- [x] Go module, project layout (`cmd/`, `internal/modules`, `ports`, `adapters`, `bootstrap`, `platform`, `shared/`)
- [x] Port interfaces: EventBus, LLM, Email, Repository, Cache, Publisher
- [x] Domain types and domain events (`EmailFetched`, `ArticleCreated`, `EmbeddingCreated`, `ClusterUpdated`, `DigestReady`)
- [x] Viper-based config with provider selection and `NEKTAR_` env overrides
- [x] In-memory event bus, cache, and storage adapters
- [x] Redis Streams event bus (XADD, XREADGROUP, ACK, retry, DLQ) and Redis cache
- [x] Skeleton adapters: PostgreSQL, Gmail, Gemini, Discord
- [x] Bootstrap wiring, scheduler registration, handler subscriptions
- [x] Logger (slog), Prometheus metrics, health endpoint
- [x] Docker Compose (Redis + PostgreSQL), Makefile, README

---

## Phase 1 — Database & Persistence

- [ ] Define SQL migration files for core tables:
  - `emails` (id, subject, from, body, received_at, processed_at)
  - `articles` (id, email_id, title, content, url, created_at)
  - `embeddings` (id, article_id, vector, created_at)
  - `clusters` (id, name, centroid, updated_at)
  - `cluster_articles` (cluster_id, article_id)
  - `digests` (id, title, summary, article_ids, created_at, published_at)
- [ ] Add migration runner (e.g. `golang-migrate` or `goose`) and wire into bootstrap
- [ ] Implement PostgreSQL `EmailRepository` (Save, FindByID, list unprocessed)
- [ ] Implement PostgreSQL `ArticleRepository` (Save, FindByID, list by email/cluster)
- [ ] Implement PostgreSQL `DigestRepository` (Save, FindByID, list recent)
- [ ] Add repository list/query methods needed by pipeline modules (extend ports as needed)
- [ ] Add SQLite adapter for local development (optional, per LLD)

---

## Phase 2 — Core Pipeline (Business Modules)

Each module should: receive event → load/save via repositories → call ports → publish next event.

### Fetcher (scheduler-driven)

- [ ] Call `email.Provider.Fetch()` on schedule
- [ ] Deduplicate against stored emails (by message ID)
- [ ] Persist new emails via `EmailRepository`
- [ ] Publish `EmailFetched` for each new email
- [ ] Handle fetch errors with logging and metrics

### Extractor (`EmailFetched`)

- [ ] Load email by ID from repository
- [ ] Parse newsletter HTML/plain text into one or more `Article` records
- [ ] Persist articles
- [ ] Publish `ArticleCreated` per article
- [ ] Mark email as processed

### Embedding (`ArticleCreated`)

- [ ] Load article by ID
- [ ] Call `llm.Provider.Embed()` on article content
- [ ] Persist `Embedding` record
- [ ] Publish `EmbeddingCreated`
- [ ] Cache embeddings in `Cache` for fast re-clustering (optional)

### Clustering (`EmbeddingCreated`)

- [ ] Load embedding and existing clusters
- [ ] Assign article to nearest cluster (cosine similarity) or create new cluster
- [ ] Update cluster centroid and `cluster_articles` join table
- [ ] Publish `ClusterUpdated` when cluster membership changes

### Digest (`ClusterUpdated`)

- [ ] Load cluster and its articles
- [ ] Call `llm.Provider.Summarize()` to produce digest summary
- [ ] Create `Digest` record linked to cluster articles
- [ ] Publish `DigestReady`

### Publisher (`DigestReady`)

- [ ] Load digest by ID
- [ ] Call `publisher.Publisher.Publish()` (Discord webhook)
- [ ] Mark digest as published
- [ ] Record publish metrics

---

## Phase 3 — Default Adapter Implementations

### Gmail (`internal/adapters/gmail`)

- [ ] Implement `Fetch()`: list messages matching newsletter label/query
- [ ] Parse Gmail message format (headers, MIME parts, base64 body)
- [ ] Map to `domain.Email`
- [ ] Support pagination and incremental fetch (since last run)
- [ ] Add configurable Gmail search query in config (e.g. `label:newsletters`)

### Gemini (`internal/adapters/gemini`)

- [ ] Implement `Embed()` using configured embed model
- [ ] Implement `Summarize()` with a digest-focused prompt template
- [ ] Handle rate limits and retries
- [ ] Add token/length truncation for large articles

### Discord (`internal/adapters/discord`)

- [ ] Implement `Publish()`: format digest as Discord embed or markdown message
- [ ] Use existing `sendWebhook()` helper
- [ ] Truncate content to Discord limits (2000 chars / embed fields)

### PostgreSQL (`internal/adapters/postgres`)

- [ ] Replace all `ErrNotImplemented` stubs with real SQL queries
- [ ] Use parameterized queries (pgx)
- [ ] Add connection pool tuning in config

---

## Phase 4 — Auth & Credential Flows

- [ ] Document OAuth2 setup for Gmail (Google Cloud Console, scopes, refresh token)
- [ ] Add `cmd/nektar-auth` CLI or bootstrap subcommand to obtain Gmail refresh token
- [ ] Validate required credentials at startup with clear error messages
- [ ] Support credential files / secrets manager paths in config
- [ ] Add `.env.example` with all required variables

---

## Phase 5 — Observability & Reliability

- [ ] Instrument all module handlers with Prometheus counters (published, handled, errors)
- [ ] Add structured logging with correlation IDs (email_id, article_id, digest_id)
- [ ] Add graceful shutdown: drain event bus consumers before exit
- [ ] Redis DLQ monitoring: alert or CLI to inspect/replay dead-letter messages
- [ ] Add readiness probe that checks Redis + Postgres connectivity
- [ ] Configurable retry/backoff policies per handler

---

## Phase 6 — Testing

- [ ] Unit tests for each module with in-memory bus + in-memory storage
- [ ] Integration tests for Redis event bus (requires Docker)
- [ ] Integration tests for PostgreSQL repositories (requires Docker)
- [ ] Contract tests for Gmail/Gemini/Discord adapters (mocked HTTP)
- [ ] End-to-end test: inject `EmailFetched` → assert `DigestReady` published (in-memory)
- [ ] Add `make test-integration` target with Docker Compose test profile

---

## Phase 7 — Additional Adapters (Future)

Per LLD, these are not required for MVP but should be added as separate adapters behind existing ports.

### Event Bus

- [ ] Kafka adapter (`platform/eventbus/kafka`)
- [ ] NATS adapter (`platform/eventbus/nats`)

### Email

- [ ] Outlook adapter
- [ ] IMAP adapter

### LLM

- [ ] OpenAI adapter
- [ ] Claude adapter
- [ ] Ollama (local) adapter

### Publisher

- [ ] Slack adapter
- [ ] Telegram adapter
- [ ] Email (SMTP) adapter
- [ ] RSS feed adapter

### Storage

- [ ] SQLite adapter (local dev)

---

## Phase 8 — Deployment & Production

- [ ] Production Dockerfile (multi-stage build)
- [ ] Kubernetes manifests or Terraform (optional)
- [ ] CI pipeline: lint, test, build, push image
- [ ] Production config templates (secrets via env / K8s secrets)
- [ ] Runbook: startup, credential rotation, DLQ replay, disaster recovery
- [ ] Rate limiting and cost controls for LLM API calls

---

## Suggested Implementation Order

1. **Phase 1** — Database schema and PostgreSQL repos (everything else depends on persistence)
2. **Phase 3** — Gmail Fetch, Gemini Embed/Summarize, Discord Publish (unblock real data flow)
3. **Phase 2** — Pipeline modules in order: Fetcher → Extractor → Embedding → Clustering → Digest → Publisher
4. **Phase 4** — Auth CLI and credential validation
5. **Phase 6** — Tests alongside each module (don't defer all testing to the end)
6. **Phase 5** — Observability and reliability hardening
7. **Phase 8** — Production deployment
8. **Phase 7** — Additional adapters as needed

---

## Definition of Done (MVP)

The project is **MVP-complete** when:

1. Scheduler fetches newsletter emails from Gmail on a configured interval
2. Pipeline extracts articles, generates embeddings, clusters them, and produces a digest
3. Digest is published to Discord automatically
4. All state is persisted in PostgreSQL
5. Events flow through Redis Streams in production (in-memory for tests)
6. Integration tests pass with Docker Compose
7. README documents setup end-to-end including Gmail OAuth and API keys
