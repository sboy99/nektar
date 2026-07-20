# Nektar — Install, Setup & User Guide

Nektar turns newsletter email into a daily tech digest on Discord.

```
Gmail → detect newsletters → extract articles → embed → cluster → summarize → Discord
```

This guide covers install, first-time setup, day-to-day use, tuning, and troubleshooting.

---

## 1. Prerequisites

| Requirement | Notes |
|-------------|--------|
| Go **1.25+** | Needed to build from source |
| Docker & Docker Compose | Local Postgres + Redis |
| Google Cloud project | Gmail API + OAuth client |
| Gemini API key | Embeddings + summarization |
| Discord webhook | Destination channel for digests |

Optional: a GitHub release binary if you prefer not to build from source.

---

## 2. Install

### Option A — Build from source

```bash
git clone https://github.com/sboy99/nektar.git
cd nektar

make deps
make build
```

Binaries:

| Binary | Path | Purpose |
|--------|------|---------|
| `nektar` | `bin/nektar` | Main pipeline service |
| `nektar-gmail-auth` | build with `go build -o bin/nektar-gmail-auth ./cmd/nektar-gmail-auth` | One-time Gmail OAuth helper |

### Option B — Release archive

1. Download the archive for your OS/arch from the [GitHub Releases](https://github.com/sboy99/nektar/releases) page.
2. Extract it. You get `nektar`, `nektar-gmail-auth`, `README.md`, and a sample `config.yaml`.
3. Copy and edit the config:

```bash
cp config.yaml configs/config.yaml   # or keep beside the binary
./nektar -config configs/config.yaml
```

---

## 3. Setup

### 3.1 Start infrastructure

```bash
make infra-up
```

This starts:

| Service | Default |
|---------|---------|
| PostgreSQL | `localhost:5432` — user/password/db: `nektar` |
| Redis | `localhost:6379` |

Migrations run automatically the first time Nektar connects to Postgres.

Stop infra later with `make infra-down`.

### 3.2 Configure Nektar

Copy or edit `configs/config.yaml`. Production defaults use Redis + Postgres + Gmail + Gemini + Discord.

**Minimum secrets to set** in `.env` (copy from `.env.example`):

```bash
cp .env.example .env
```

```bash
NEKTAR_GMAIL_CLIENT_ID=...
NEKTAR_GMAIL_CLIENT_SECRET=...
NEKTAR_GEMINI_API_KEY=...
NEKTAR_DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
```

Per-user Gmail **refresh tokens** are stored in Postgres (`gmail_sync.refresh_token`), not in `.env`.

`configs/config.yaml` holds non-secret settings only. On startup, `config.Load` reads `.env` then `.env.local` from the working directory (optional override: `NEKTAR_ENV_FILE`). Existing OS env vars always win.

You can still export the same `NEKTAR_*` variables in your shell instead of using a file.

### 3.3 Gmail OAuth

1. In [Google Cloud Console](https://console.cloud.google.com/):
   - Create (or pick) a project
   - Enable **Gmail API**
   - Create an OAuth 2.0 Client ID (Desktop or Web)
   - Add redirect URI: `http://localhost:8085/oauth2/callback`
2. Run the helper:

```bash
go run ./cmd/nektar-gmail-auth
```

3. Open the printed URL, approve access (Gmail readonly + profile/email scopes).
4. The helper fetches your Google profile and upserts `users` + `gmail_sync` (UUID `users.id`, FK on `gmail_sync.user_id`).

### 3.4 Discord webhook

1. Discord channel → **Edit Channel** → **Integrations** → **Webhooks** → **New Webhook**
2. Copy the webhook URL into `NEKTAR_DISCORD_WEBHOOK_URL` in `.env`

### 3.5 Seed a user + Gmail sync cursor

Nektar fetches for every row in `gmail_sync`. Prefer `nektar-gmail-auth` (it writes the rows). Manual seed example:

```bash
psql 'postgres://nektar:nektar@localhost:5432/nektar?sslmode=disable' <<'SQL'
INSERT INTO users (id, email, name, google_id, avatar_url, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111', 'you@example.com', 'You', '', '', now(), now());

INSERT INTO gmail_sync (user_id, history_id, last_synced_at, query, refresh_token)
VALUES ('11111111-1111-1111-1111-111111111111', '', now(), 'newer_than:7d', '1//0g...');
SQL
```

| Field | Meaning |
|-------|---------|
| `refresh_token` | Gmail OAuth refresh token for this mailbox |
| `query` | Gmail search for the first bootstrap sync |
| `history_id` | Leave empty for the first run; History API fills it afterward |

### 3.6 Run

```bash
make run
# or
./bin/nektar -config configs/config.yaml
```

On startup Nektar:

1. Connects to Postgres/Redis (or in-memory providers)
2. Subscribes pipeline workers to Redis Streams
3. Runs scheduled jobs once immediately, then on intervals
4. Serves HTTP on `:8080` (`/health`, `/ready`, `/metrics`)

---

## 4. User guide

### 4.1 What happens day to day

| Stage | What Nektar does |
|-------|------------------|
| **Fetch** | Pulls new mail for each `gmail_sync` user |
| **Detect** | Keeps likely newsletters; rejects the rest |
| **Extract** | Cleans HTML/MIME, splits multi-story issues into articles |
| **Embed** | Creates vectors (Gemini) and caches by content hash |
| **Cluster** | Groups similar articles into topics |
| **Digest** | When enough recent signal exists, builds a markdown digest |
| **Publish** | Posts the digest to Discord via webhook |

You do not drive stages manually. Keep the process running and tune thresholds if digests are too rare/noisy.

### 4.2 Scheduled jobs

Jobs run on startup, then on these intervals (`scheduler:` in config):

| Job | Default | Purpose |
|-----|---------|---------|
| `fetch_gmail` | `15m` | Fetch new email |
| `retry_failures` | `5m` | Replay failed events from the DLQ |
| `publish_digest` | `1h` | Publish any ready digests that were missed |
| `cleanup_cache` | `24h` | Purge expired cache entries |
| `cleanup_old_events` | `24h` | Trim old streams / emails past retention |
| `refresh_oauth` | `12h` | Probe/refresh Gmail tokens |

### 4.3 Digest behavior

Digests are created when recent clustered articles meet thresholds:

```yaml
digest:
  lookback: 24h
  min_articles: 3
  min_clusters: 1
  min_cluster_size: 1
  max_clusters: 10
  max_articles_per_cluster: 5
```

Tips:

- **Few digests** → lower `min_articles` / widen `lookback` / widen Gmail `query`
- **Noisy digests** → raise `min_articles`, tighten newsletter allowlist, raise clustering thresholds
- Prompt templates live in `prompts/` (`digest.prompts_dir`)

### 4.4 Newsletter filtering

```yaml
newsletter:
  score_threshold: 0.5
  allowlist: ["news@tldr.tech", "@substack.com"]
  denylist: []
```

- Allowlist entries can be a full address or a domain suffix (`@substack.com`)
- Detection is heuristic (headers / bulk signals); no LLM call at this stage

### 4.5 Monitoring

| Endpoint | Use |
|----------|-----|
| `GET /health` | Process is up |
| `GET /ready` | Postgres + Redis/cache are reachable |
| `GET /metrics` | Prometheus metrics |

Logs are structured JSON. Set `logging.level` to `debug` while diagnosing pipeline issues. Events carry a `correlation_id` across stages.

### 4.6 Local / offline-ish development

For adapter-light local runs (still needs a Gemini key to wire LLM providers):

```yaml
eventbus:
  provider: inmemory
storage:
  provider: inmemory
cache:
  provider: inmemory
```

Unit + in-memory E2E tests:

```bash
make test
```

Integration tests (need infra):

```bash
make infra-up
make test-integration
```

### 4.7 Multi-user

Each Gmail mailbox is a `users` + `gmail_sync` pair with its own `refresh_token`.

1. Run `nektar-gmail-auth` for each mailbox (reuses UUID by email, or generates a new one)
2. Confirm `users` + `gmail_sync` rows were written

Digests and Discord posts are produced per user pipeline activity.

---

## 5. Configuration reference (common knobs)

| Area | Keys | Effect |
|------|------|--------|
| Providers | `eventbus/storage/cache/llm/email/publisher.provider` | Swap implementations |
| Redis | `redis.addr`, `stream_prefix`, `group_name` | Streams + cache |
| Postgres | `postgres.dsn` | Primary store |
| Clustering | `similarity_threshold`, `merge_threshold` | Topic grouping strictness |
| Embedding | `embedding.cache_ttl`, `gemini.embed_*` | Vector model + cache |
| Server | `server.addr` | HTTP listen address (default `:8080`) |
| Retry | `retry.*` | Event bus / fetch backoff |

Full sample: [`configs/config.yaml`](../configs/config.yaml).

---

## 6. Troubleshooting

| Symptom | Check |
|---------|--------|
| Process exits on start | Config path, Gemini key, DSN, Redis addr; read stderr |
| `/ready` fails | `make infra-up`; verify DSN / Redis |
| No emails fetched | `gmail_sync` row exists with non-empty `refresh_token`; OAuth client + Gmail API enabled |
| OAuth helper fails | Redirect URI exact match; client ID/secret; port `8085` free |
| Emails fetched but nothing extracted | Newsletter score / denylist; try allowlisting known senders |
| Articles but no digest | `digest.min_*` / `lookback`; wait for clustering; check logs for digest module |
| Digest ready but no Discord post | `discord.webhook_url`; channel permissions; publisher logs / metrics |
| Duplicate or stale posts | `publish_interval`; digest publish status in DB |

Useful log filter: search for module names (`fetcher`, `newsletter`, `extractor`, `embedding`, `clustering`, `digest`, `publisher`) and `correlation_id`.

---

## 7. Makefile cheatsheet

| Command | Description |
|---------|-------------|
| `make deps` | `go mod tidy` |
| `make build` | Build `bin/nektar` |
| `make run` | Build + run with `configs/config.yaml` |
| `make test` | Unit / E2E (no infra) |
| `make test-integration` | Postgres + Redis adapter tests |
| `make lint` | `gofmt` + `go vet` |
| `make hooks` | Enable repo git hooks |
| `make infra-up` / `infra-down` | Start/stop Docker services |
| `make clean` | Remove `bin/` |

---

## 8. Security notes

- Store OAuth secrets and API keys in `.env` / a secret manager — not in git or `configs/config.yaml`
- Gmail scope is **read-only** (`gmail.readonly`)
- Discord webhooks are channel credentials; rotate if leaked
- Prefer TLS-capable Postgres/Redis DSNs outside local Docker

For architecture and roadmap, see [README.md](../README.md) and [TASKS.md](../TASKS.md).
