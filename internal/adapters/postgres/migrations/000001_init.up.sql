CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE gmail_sync (
    user_id           TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    history_id        TEXT NOT NULL DEFAULT '',
    last_synced_at    TIMESTAMPTZ NOT NULL,
    query             TEXT NOT NULL DEFAULT '',
    refresh_token_ref TEXT NOT NULL DEFAULT ''
);

CREATE TABLE emails (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    gmail_message_id  TEXT NOT NULL,
    thread_id         TEXT NOT NULL DEFAULT '',
    subject           TEXT NOT NULL DEFAULT '',
    "from"            TEXT NOT NULL DEFAULT '',
    raw_body          TEXT NOT NULL DEFAULT '',
    headers           JSONB NOT NULL DEFAULT '{}',
    received_at       TIMESTAMPTZ NOT NULL,
    is_newsletter     BOOLEAN NOT NULL DEFAULT FALSE,
    newsletter_score  DOUBLE PRECISION NOT NULL DEFAULT 0,
    list_id           TEXT NOT NULL DEFAULT '',
    list_unsubscribe  TEXT NOT NULL DEFAULT '',
    stage             TEXT NOT NULL DEFAULT 'fetched',
    UNIQUE (user_id, gmail_message_id)
);

CREATE INDEX emails_user_stage_idx ON emails (user_id, stage);

CREATE TABLE articles (
    id                   TEXT PRIMARY KEY,
    user_id              TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email_id             TEXT NOT NULL REFERENCES emails (id) ON DELETE CASCADE,
    title                TEXT NOT NULL DEFAULT '',
    raw_html             TEXT NOT NULL DEFAULT '',
    markdown             TEXT NOT NULL DEFAULT '',
    plain_text           TEXT NOT NULL DEFAULT '',
    url                  TEXT NOT NULL DEFAULT '',
    reading_time_minutes INTEGER NOT NULL DEFAULT 0,
    position             INTEGER NOT NULL DEFAULT 0,
    stage                TEXT NOT NULL DEFAULT 'extracted',
    created_at           TIMESTAMPTZ NOT NULL
);

CREATE INDEX articles_user_stage_idx ON articles (user_id, stage);
CREATE INDEX articles_email_id_idx ON articles (email_id);

CREATE TABLE clusters (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL DEFAULT '',
    centroid   REAL[],
    topic_id   TEXT,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX clusters_user_id_idx ON clusters (user_id);

CREATE TABLE cluster_articles (
    cluster_id TEXT NOT NULL REFERENCES clusters (id) ON DELETE CASCADE,
    article_id TEXT NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    PRIMARY KEY (cluster_id, article_id)
);

CREATE INDEX cluster_articles_article_id_idx ON cluster_articles (article_id);

CREATE TABLE embeddings (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    article_id TEXT NOT NULL UNIQUE REFERENCES articles (id) ON DELETE CASCADE,
    vector     REAL[] NOT NULL,
    model      TEXT NOT NULL DEFAULT '',
    provider   TEXT NOT NULL DEFAULT '',
    dimensions INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE digests (
    id                   TEXT PRIMARY KEY,
    user_id              TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title                TEXT NOT NULL DEFAULT '',
    markdown             TEXT NOT NULL DEFAULT '',
    summary              TEXT NOT NULL DEFAULT '',
    article_ids          TEXT[] NOT NULL DEFAULT '{}',
    cluster_ids          TEXT[] NOT NULL DEFAULT '{}',
    reading_time_minutes INTEGER NOT NULL DEFAULT 0,
    publish_status       TEXT NOT NULL DEFAULT 'draft',
    published_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL
);

CREATE INDEX digests_user_status_created_idx ON digests (user_id, publish_status, created_at DESC);

CREATE TABLE llm_requests (
    id                 TEXT PRIMARY KEY,
    user_id            TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    resource_type      TEXT NOT NULL DEFAULT '',
    resource_id        TEXT NOT NULL DEFAULT '',
    provider           TEXT NOT NULL DEFAULT '',
    model              TEXT NOT NULL DEFAULT '',
    tokens             INTEGER NOT NULL DEFAULT 0,
    latency_ms         BIGINT NOT NULL DEFAULT 0,
    estimated_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX llm_requests_user_created_idx ON llm_requests (user_id, created_at);
