-- Convert users.id and all user_id FKs from TEXT to UUID.
-- Non-UUID legacy ids are rewritten via a stable mapping to gen_random_uuid().

CREATE TEMP TABLE user_id_map ON COMMIT DROP AS
SELECT
    id AS old_id,
    CASE
        WHEN id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN id::uuid
        ELSE gen_random_uuid()
    END AS new_id
FROM users;

ALTER TABLE gmail_sync DROP CONSTRAINT IF EXISTS gmail_sync_user_id_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_user_id_fkey;
ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_user_id_fkey;
ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_user_id_fkey;
ALTER TABLE embeddings DROP CONSTRAINT IF EXISTS embeddings_user_id_fkey;
ALTER TABLE digests DROP CONSTRAINT IF EXISTS digests_user_id_fkey;
ALTER TABLE llm_requests DROP CONSTRAINT IF EXISTS llm_requests_user_id_fkey;

ALTER TABLE users ADD COLUMN id_uuid UUID;
UPDATE users u
SET id_uuid = m.new_id
FROM user_id_map m
WHERE u.id = m.old_id;

ALTER TABLE gmail_sync ADD COLUMN user_id_uuid UUID;
UPDATE gmail_sync g
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE g.user_id = m.old_id;

ALTER TABLE emails ADD COLUMN user_id_uuid UUID;
UPDATE emails e
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE e.user_id = m.old_id;

ALTER TABLE articles ADD COLUMN user_id_uuid UUID;
UPDATE articles a
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE a.user_id = m.old_id;

ALTER TABLE clusters ADD COLUMN user_id_uuid UUID;
UPDATE clusters c
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE c.user_id = m.old_id;

ALTER TABLE embeddings ADD COLUMN user_id_uuid UUID;
UPDATE embeddings emb
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE emb.user_id = m.old_id;

ALTER TABLE digests ADD COLUMN user_id_uuid UUID;
UPDATE digests d
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE d.user_id = m.old_id;

ALTER TABLE llm_requests ADD COLUMN user_id_uuid UUID;
UPDATE llm_requests r
SET user_id_uuid = m.new_id
FROM user_id_map m
WHERE r.user_id = m.old_id;

ALTER TABLE gmail_sync DROP CONSTRAINT gmail_sync_pkey;
ALTER TABLE gmail_sync DROP COLUMN user_id;
ALTER TABLE gmail_sync RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE gmail_sync ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE gmail_sync ADD PRIMARY KEY (user_id);

ALTER TABLE emails DROP COLUMN user_id;
ALTER TABLE emails RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE emails ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE articles DROP COLUMN user_id;
ALTER TABLE articles RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE articles ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE clusters DROP COLUMN user_id;
ALTER TABLE clusters RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE clusters ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE embeddings DROP COLUMN user_id;
ALTER TABLE embeddings RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE embeddings ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE digests DROP COLUMN user_id;
ALTER TABLE digests RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE digests ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE llm_requests DROP COLUMN user_id;
ALTER TABLE llm_requests RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE llm_requests ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users DROP COLUMN id;
ALTER TABLE users RENAME COLUMN id_uuid TO id;
ALTER TABLE users ALTER COLUMN id SET NOT NULL;
ALTER TABLE users ADD PRIMARY KEY (id);

ALTER TABLE gmail_sync
    ADD CONSTRAINT gmail_sync_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE emails
    ADD CONSTRAINT emails_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE articles
    ADD CONSTRAINT articles_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE clusters
    ADD CONSTRAINT clusters_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE embeddings
    ADD CONSTRAINT embeddings_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE digests
    ADD CONSTRAINT digests_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE llm_requests
    ADD CONSTRAINT llm_requests_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE emails
    ADD CONSTRAINT emails_user_id_gmail_message_id_key UNIQUE (user_id, gmail_message_id);

CREATE INDEX emails_user_stage_idx ON emails (user_id, stage);
CREATE INDEX articles_user_stage_idx ON articles (user_id, stage);
CREATE INDEX clusters_user_id_idx ON clusters (user_id);
CREATE INDEX digests_user_status_created_idx ON digests (user_id, publish_status, created_at DESC);
CREATE INDEX llm_requests_user_created_idx ON llm_requests (user_id, created_at);
