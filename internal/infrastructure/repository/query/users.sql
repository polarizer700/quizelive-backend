-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, email_verified_at)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password_hash, role, email_verified_at, created_at;

-- name: GetUserByID :one
SELECT id, email, password_hash, role, email_verified_at, created_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, role, email_verified_at, created_at
FROM users
WHERE email = $1;

-- name: ListUsers :many
SELECT id, email, password_hash, role, email_verified_at, created_at
FROM users
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT count(*)::bigint AS total
FROM users;

-- name: UpdateUser :one
UPDATE users
SET email = $2,
    password_hash = $3,
    role = $4
WHERE id = $1
RETURNING id, email, password_hash, role, email_verified_at, created_at;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: MarkUserEmailVerified :exec
UPDATE users
SET email_verified_at = $2
WHERE id = $1;

-- name: DeletePendingVerifications :exec
DELETE FROM email_verifications
WHERE user_id = $1
  AND used_at IS NULL;

-- name: InsertEmailVerification :exec
INSERT INTO email_verifications (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- name: ConsumeEmailVerification :one
UPDATE email_verifications
SET used_at = $2
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > $2
RETURNING user_id;

-- name: GetEmailVerificationByHash :one
SELECT user_id, used_at
FROM email_verifications
WHERE token_hash = $1;

-- name: DeletePendingPasswordResets :exec
DELETE FROM password_resets
WHERE user_id = $1
  AND used_at IS NULL;

-- name: InsertPasswordReset :exec
INSERT INTO password_resets (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- name: ConsumePasswordReset :one
UPDATE password_resets
SET used_at = $2
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > $2
RETURNING user_id;
