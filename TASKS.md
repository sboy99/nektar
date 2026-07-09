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

Remaining work is primarily business logic, persistence, production adapters, testing, and deployment.

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

Finalize the business model before implementing persistence.

## Tasks

- [ ] Finalize aggregate boundaries
- [ ] Email entity
- [ ] Article entity
- [ ] Cluster entity
- [ ] Digest entity
- [ ] User entity
- [ ] Embedding entity
- [ ] Topic entity
- [ ] Domain errors
- [ ] Repository interfaces
- [ ] Domain validation
- [ ] Pipeline status model

---

# Phase 1 — Persistence

## Database Schema

### Core Tables

- [ ] `users`
- [ ] `gmail_sync`
- [ ] `emails`
- [ ] `articles`
- [ ] `clusters`
- [ ] `cluster_articles`
- [ ] `digests`
- [ ] `llm_requests`

### Optional

- [ ] `topics`
- [ ] `pipeline_status`

## Migration

- [ ] Add golang-migrate
- [ ] Bootstrap migrations
- [ ] Rollback support

## PostgreSQL

Implement:

- [ ] `UserRepository`
- [ ] `EmailRepository`
- [ ] `ArticleRepository`
- [ ] `ClusterRepository`
- [ ] `DigestRepository`

## Future

- [ ] SQLite adapter

---

# Phase 2 — Fetch Pipeline

## Goal

Synchronize Gmail incrementally.

## Gmail Adapter

- [ ] OAuth
- [ ] Refresh token
- [ ] History API
- [ ] Incremental sync
- [ ] Configurable Gmail query

## Fetcher Module

- [ ] Fetch emails
- [ ] Deduplicate
- [ ] Store
- [ ] Publish `EmailFetched`
- [ ] Metrics
- [ ] Retry

---

# Phase 3 — Newsletter Detection

## Goal

Ignore everything that is not a newsletter.

## Tasks

- [ ] List-ID parsing
- [ ] Bulk header detection
- [ ] List-Unsubscribe parsing
- [ ] Sender allowlist
- [ ] Sender denylist
- [ ] Spam filtering
- [ ] Newsletter classification

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

- [ ] MIME parser
- [ ] HTML parser
- [ ] Plain text parser

## Normalizer

- [ ] Remove navigation
- [ ] Remove footer
- [ ] Remove ads
- [ ] Remove unsubscribe
- [ ] Normalize whitespace

## Splitter

- [ ] Detect multiple articles
- [ ] Persist articles
- [ ] Publish `ArticleCreated`

---

# Phase 5 — Embedding Pipeline

## Providers

Separate ports — do **not** combine both.

```
EmbeddingProvider
SummaryProvider
```

## Tasks

- [ ] Generate embeddings
- [ ] Persist embedding
- [ ] Cache embedding
- [ ] Publish `EmbeddingCreated`

## Cost Tracking

Persist:

- [ ] tokens
- [ ] latency
- [ ] provider
- [ ] model
- [ ] estimated cost

---

# Phase 6 — Topic Clustering

## Goal

Convert articles into topics.

## Tasks

- [ ] Nearest-neighbor search
- [ ] Cosine similarity
- [ ] Create cluster
- [ ] Update centroid
- [ ] Merge clusters
- [ ] Publish `ClusterUpdated`

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

- [ ] Importance scoring
- [ ] Ranking
- [ ] Topic ordering
- [ ] Reading time

## Summary

- [ ] Prompt templates
- [ ] LLM summary
- [ ] Markdown rendering

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

- [ ] Markdown builder
- [ ] Embed builder
- [ ] Webhook client
- [ ] Retry
- [ ] Publish metrics

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

- [ ] Emails fetched
- [ ] Newsletters detected
- [ ] Articles extracted
- [ ] Embeddings generated
- [ ] Clusters created
- [ ] Duplicate articles
- [ ] Digest latency
- [ ] Publish latency
- [ ] LLM latency
- [ ] Token usage

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
- [ ] Newsletter detector filters non-newsletters
- [ ] Articles are extracted and normalized
- [ ] Embeddings are generated
- [ ] Articles are clustered
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
