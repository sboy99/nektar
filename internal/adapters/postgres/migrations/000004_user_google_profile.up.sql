ALTER TABLE users
    ADD COLUMN google_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX users_google_id_uidx ON users (google_id) WHERE google_id <> '';
