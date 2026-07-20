DROP INDEX IF EXISTS users_google_id_uidx;

ALTER TABLE users
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS google_id;
