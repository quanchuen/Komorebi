ALTER TABLE community.user
    ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
