-- name: CreateQuiz :one
INSERT INTO quizzes (title, description, owner_id, status, game_type, break_after_round)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, title, description, owner_id, status, game_type, break_after_round, created_at, updated_at;

-- name: GetQuiz :one
SELECT id, title, description, owner_id, status, game_type, break_after_round, created_at, updated_at
FROM quizzes
WHERE id = $1;

-- name: ListQuizzes :many
SELECT id, title, description, owner_id, status, game_type, break_after_round, created_at, updated_at
FROM quizzes
WHERE (sqlc.narg('owner_id')::uuid IS NULL OR owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg('row_limit') OFFSET sqlc.arg('row_offset');

-- name: CountQuizzes :one
SELECT count(*)::bigint AS total
FROM quizzes
WHERE (sqlc.narg('owner_id')::uuid IS NULL OR owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'));

-- name: UpdateQuiz :one
UPDATE quizzes
SET title = $2,
    description = $3,
    status = $4,
    game_type = $5,
    break_after_round = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, title, description, owner_id, status, game_type, break_after_round, created_at, updated_at;

-- name: DeleteQuiz :exec
DELETE FROM quizzes
WHERE id = $1;

-- name: ListPublicGames :many
SELECT q.id, q.title, q.description, q.game_type, q.created_at,
       (SELECT COUNT(*) FROM questions WHERE quiz_id = q.id)::bigint AS questions_count
FROM quizzes q
WHERE q.status = 'published'
ORDER BY q.created_at DESC
LIMIT sqlc.arg('row_limit') OFFSET sqlc.arg('row_offset');

-- name: CountPublicGames :one
SELECT COUNT(*)::bigint
FROM quizzes
WHERE status = 'published';
