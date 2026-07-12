ALTER TABLE gmail_sync
    ADD COLUMN refresh_token_ref TEXT NOT NULL DEFAULT '';

ALTER TABLE gmail_sync
    DROP COLUMN refresh_token;
