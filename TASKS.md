# Nektar — Project Tasks

Track the remaining work required to evolve Nektar from the current foundation scaffold into a production-ready, event-driven technical intelligence platform.

Reference:
- [.cursor/lld.md](.cursor/lld.md)

---

# Current Status

## Foundation Complete

The project already contains:

- Hexagonal Architecture
- Ports & Adapters
- Config-driven bootstrap
- Redis Streams EventBus
- InMemory implementations
- Provider abstraction
- Scheduler
- Health endpoint
- Metrics
- Logger
- Docker Compose
- Bootstrap wiring

## Phase 0 Complete

The domain model is finalized (multi-user). See `shared/domain/` for all entities, validation, and aggregate boundaries.

## Phase 1 Complete

PostgreSQL schema, migrations, and repository implementations are in place (plus in-memory adapters).

## Phase 2 Complete

Multi-user Gmail fetch pipeline: History API incremental sync, per-user refresh tokens, fetcher dedupe/store/`EmailFetched`, metrics, and retry.

## Phase 3 Complete

Heuristic newsletter detection: List-ID / List-Unsubscribe / bulk headers, allow/deny lists, spam filtering, `EmailDetected` event for confirmed newsletters.

## Phase 4 Complete

Extraction pipeline: MIME/HTML/plain parsing, HTML normalization, markdown conversion, heuristic article splitting, persistence, and `ArticleCreated` events.

## Phase 5 Complete

Embedding pipeline: Gemini EmbedContent, content-addressed cache, persistence, LLMRequest cost tracking, and `EmbeddingCreated` events.

## Phase 6 Complete

Topic clustering: nearest-neighbor assignment, centroid updates, merge, and `ClusterUpdated` events.

## Phase 7 Complete

Digest builder: importance ranking/filtering, file-based prompts, Gemini summarization, markdown digests, and `DigestReady` events.

## Phase 8 Complete

Discord publisher: markdown/embed builders, webhook client with retry, digest status updates, and publish metrics.

Remaining work is primarily scheduling, testing, and deployment.

---

# Project Pipeline

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

Every phase below corresponds to one stage in this pipeline.

---

# Phase 0 — Domain Model

## Goal

Finalize the business model before implementing persistence. Multi-user from day one: `User` is a first-class aggregate; `user_id` appears on all user-owned entities.

## Aggregate Boundaries

| Aggregate | Root | Notes |
|-----------|------|-------|
| User | `User` | Gmail OAuth credentials, sync cursor |
| Email | `Email` | Raw fetched message; newsletter detection state |
| Article | `Article` | Normalized/split content |
| Cluster | `Cluster` | Vector grouping; centroid + article IDs |
| Topic | `Topic` | Human-facing label for a cluster |
| Digest | `Digest` | Ranked summary ready to publish |

Entities (not aggregate roots): `Embedding`, `LLMRequest`, `PipelineStatus`

## Domain Files (`shared/domain/`)

- [x] `user.go` — User, GmailSync
- [x] `email.go` — Email
- [x] `article.go` — Article, ContentFormat
- [x] `embedding.go` — Embedding
- [x] `cluster.go` — Cluster
- [x] `topic.go` — Topic
- [x] `digest.go` — Digest, PublishStatus
- [x] `pipeline.go` — PipelineStage, PipelineStatus, ResourceType
- [x] `llm_request.go` — LLMRequest
- [x] `errors.go` — domain sentinel errors
- [x] `validation.go` — Validate() methods and constructors
- [x] `doc.go` — package documentation

## Tasks

- [x] Finalize aggregate boundaries
- [x] Email entity
- [x] Article entity
- [x] Cluster entity
- [x] Digest entity
- [x] User entity
- [x] Embedding entity
- [x] Topic entity
- [x] Domain errors
- [x] Repository interfaces
- [x] Domain validation
- [x] Pipeline status model

## Port Updates

- [x] Split `llm.Provider` into `embedding.Provider` and `summary.Provider`
- [x] Extend repository ports: User, Cluster, Embedding, LLMRequest
- [x] Add `UserID` to all event payloads in `shared/events/`

## Definition of Done

- [x] All entities defined with multi-user `user_id` fields
- [x] Domain validation tests pass (`go test ./shared/domain/...`)
- [x] All ports compile; adapters stub new repository methods
- [x] No SQL migrations or PostgreSQL query implementation started

---

# Phase 1 — Persistence

## Database Schema

### Core Tables

- [x] `users`
- [x] `gmail_sync`
- [x] `emails`
- [x] `articles`
- [x] `clusters`
- [x] `cluster_articles`
- [x] `embeddings` (required by `EmbeddingRepository`; vectors as `REAL[]`)
- [x] `digests`
- [x] `llm_requests`

### Optional (deferred — no repository ports yet; stage lives on Email/Article)

- [ ] `topics`
- [ ] `pipeline_status`

## Migration

- [x] Add golang-migrate
- [x] Bootstrap migrations (auto `Up()` in `postgres.NewStorage`)
- [x] Rollback support (`000001_init.down.sql`)

## PostgreSQL

Implement:

- [x] `UserRepository`
- [x] `EmailRepository`
- [x] `ArticleRepository`
- [x] `ClusterRepository`
- [x] `DigestRepository`
- [x] `EmbeddingRepository`
- [x] `LLMRequestRepository`

## Future

- [ ] SQLite adapter

---

# Phase 2 — Fetch Pipeline

## Goal

Synchronize Gmail incrementally.

## Gmail Adapter

- [x] OAuth
- [x] Refresh token
- [x] History API
- [x] Incremental sync
- [x] Configurable Gmail query

## Fetcher Module

- [x] Fetch emails
- [x] Deduplicate
- [x] Store
- [x] Publish `EmailFetched`
- [x] Metrics
- [x] Retry

---

# Phase 3 — Newsletter Detection

## Goal

Ignore everything that is not a newsletter.

## Tasks

- [x] List-ID parsing
- [x] Bulk header detection
- [x] List-Unsubscribe parsing
- [x] Sender allowlist
- [x] Sender denylist
- [x] Spam filtering
- [x] Newsletter classification

## Output

```
Email → Newsletter
```

---

# Phase 4 — Extraction Pipeline

Split responsibilities.

```
Email
    │
    ▼
HTML Extraction
    │
    ▼
Normalization
    │
    ▼
Markdown
    │
    ▼
Article Splitter
    │
    ▼
Article
```

## Extractor

- [x] MIME parser
- [x] HTML parser
- [x] Plain text parser

## Normalizer

- [x] Remove navigation
- [x] Remove footer
- [x] Remove ads
- [x] Remove unsubscribe
- [x] Normalize whitespace

## Splitter

- [x] Detect multiple articles
- [x] Persist articles
- [x] Publish `ArticleCreated`

---

# Phase 5 — Embedding Pipeline

## Providers

Separate ports — do **not** combine both.

```
EmbeddingProvider
SummaryProvider
```

## Tasks

- [x] Generate embeddings
- [x] Persist embedding
- [x] Cache embedding
- [x] Publish `EmbeddingCreated`

## Cost Tracking

Persist:

- [x] tokens
- [x] latency
- [x] provider
- [x] model
- [x] estimated cost

---

# Phase 6 — Topic Clustering

## Goal

Convert articles into topics.

## Tasks

- [x] Nearest-neighbor search
- [x] Cosine similarity
- [x] Create cluster
- [x] Update centroid
- [x] Merge clusters
- [x] Publish `ClusterUpdated`

---

# Phase 7 — Digest Builder

Digest generation should be its own pipeline.

```
Clusters
    │
    ▼
Rank
    │
    ▼
Filter
    │
    ▼
Summarize
    │
    ▼
Markdown
    │
    ▼
Digest
```

## Builder

- [x] Importance scoring
- [x] Ranking
- [x] Topic ordering
- [x] Reading time

## Summary

- [x] Prompt templates
- [x] LLM summary
- [x] Markdown rendering

## Prompt Management

Store prompts separately.

```
prompts/
  summary.md
  digest.md
  cluster.md
  topic.md
```

---

# Phase 8 — Publisher

## Discord

- [x] Markdown builder
- [x] Embed builder
- [x] Webhook client
- [x] Retry
- [x] Publish metrics

## Future

- [ ] Slack
- [ ] Telegram
- [ ] Email
- [ ] RSS

---

# Phase 9 — Scheduler

Document every scheduled job.

- [ ] Fetch Gmail
- [ ] Retry failures
- [ ] Publish digest
- [ ] Cleanup cache
- [ ] Cleanup old events
- [ ] Refresh OAuth

---

# Phase 10 — Observability

## Metrics

Track:

- [x] Emails fetched
- [x] Newsletters detected
- [x] Articles extracted
- [x] Embeddings generated
- [x] Clusters created
- [ ] Duplicate articles
- [x] Digest latency
- [x] Publish latency
- [x] LLM latency
- [x] Token usage

## Logging

- [ ] Correlation IDs
- [ ] Pipeline tracing
- [ ] Structured logging

## Reliability

- [ ] Retry policy
- [ ] DLQ replay
- [ ] Readiness probe
- [ ] Graceful shutdown

---

# Phase 11 — Testing

## Unit

- [ ] Modules
- [ ] Services
- [ ] Domain

## Integration

- [ ] Redis Streams
- [ ] PostgreSQL
- [ ] Gmail adapter
- [ ] Gemini adapter
- [ ] Discord adapter

## End-to-End

```
Fetch → Extract → Embed → Cluster → Digest → Publish
```

---

# Phase 12 — Production

- [ ] Multi-stage Dockerfile
- [ ] CI
- [ ] Production config
- [ ] Kubernetes manifests
- [ ] Secrets
- [ ] Runbook
- [ ] Rate limiting
- [ ] Cost controls

---

# Phase 13 — Additional Adapters

## Event Bus

- [ ] Kafka
- [ ] NATS

## Email

- [ ] Outlook
- [ ] IMAP

## LLM

- [ ] OpenAI
- [ ] Claude
- [ ] Ollama

## Publisher

- [ ] Slack
- [ ] Telegram
- [ ] RSS
- [ ] SMTP

## Storage

- [ ] SQLite

---

# Future Roadmap

Once the MVP is complete, Nektar becomes more than a newsletter summarizer.

## Knowledge Layer

```
Email
    │
    ▼
Article
    │
    ▼
Embedding
    │
    ▼
Cluster
    │
    ▼
Knowledge Item
```

## Consumers

- Daily Digest
- Weekly Digest
- Monthly Digest
- Semantic Search
- RAG
- Podcast
- Personal Recommendations

---

# Definition of Done (MVP)

The MVP is complete when:

- [ ] Scheduler fetches newsletters from Gmail
- [x] Newsletter detector filters non-newsletters
- [x] Articles are extracted and normalized
- [ ] Embeddings are generated
- [x] Articles are clustered
- [ ] Daily digest is generated
- [ ] Digest is published to Discord
- [ ] PostgreSQL stores all state
- [ ] Redis Streams powers the event pipeline
- [ ] Integration tests pass
- [ ] Documentation covers OAuth, setup, deployment, and troubleshooting

---

# Suggested Implementation Order

1. **Phase 0** — Domain model (blocks everything else)
2. **Phase 1** — Persistence (schema + repositories)
3. **Phase 2** — Fetch pipeline (Gmail + Fetcher)
4. **Phase 3** — Newsletter detection
5. **Phase 4** — Extraction pipeline (Extractor → Normalizer → Splitter)
6. **Phase 5** — Embedding pipeline (split `EmbeddingProvider` / `SummaryProvider` ports)
7. **Phase 6** — Topic clustering
8. **Phase 7** — Digest builder + prompt management
9. **Phase 8** — Publisher (Discord)
10. **Phase 9** — Scheduler jobs
11. **Phase 10** — Observability (alongside each phase, not only at the end)
12. **Phase 11** — Testing (unit + integration per phase; E2E after pipeline is wired)
13. **Phase 12** — Production
14. **Phase 13** — Additional adapters as needed
