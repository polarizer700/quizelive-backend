-- +goose Up
CREATE TABLE password_resets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    CONSTRAINT password_resets_hash_key UNIQUE (token_hash)
);

CREATE INDEX password_resets_user_id_idx ON password_resets (user_id);

-- +goose Down
DROP TABLE IF EXISTS password_resets;
