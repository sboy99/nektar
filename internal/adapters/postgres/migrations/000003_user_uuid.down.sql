-- Revert users.id and user_id FKs from UUID back to TEXT.

ALTER TABLE gmail_sync DROP CONSTRAINT IF EXISTS gmail_sync_user_id_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_user_id_fkey;
ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_user_id_fkey;
ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_user_id_fkey;
ALTER TABLE embeddings DROP CONSTRAINT IF EXISTS embeddings_user_id_fkey;
ALTER TABLE digests DROP CONSTRAINT IF EXISTS digests_user_id_fkey;
ALTER TABLE llm_requests DROP CONSTRAINT IF EXISTS llm_requests_user_id_fkey;

ALTER TABLE gmail_sync ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE emails ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE articles ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE clusters ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE embeddings ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE digests ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE llm_requests ALTER COLUMN user_id TYPE TEXT USING user_id::text;
ALTER TABLE users ALTER COLUMN id TYPE TEXT USING id::text;

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
