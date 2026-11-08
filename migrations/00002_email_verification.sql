-- +goose Up
ALTER TABLE users
    ADD COLUMN email_verified_at timestamptz;

UPDATE users
SET email_verified_at = created_at
WHERE email_verified_at IS NULL;

CREATE TABLE email_verifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    CONSTRAINT email_verifications_hash_key UNIQUE (token_hash)
);

CREATE INDEX email_verifications_user_id_idx ON email_verifications (user_id);

-- +goose Down
DROP TABLE IF EXISTS email_verifications;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified_at;
