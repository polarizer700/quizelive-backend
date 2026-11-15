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
  AND (cardinality(sqlc.arg('statuses')::text[]) = 0 OR status = ANY(sqlc.arg('statuses')::text[]))
  AND (cardinality(sqlc.arg('game_types')::text[]) = 0 OR game_type = ANY(sqlc.arg('game_types')::text[]))
  AND (cardinality(sqlc.arg('categories')::text[]) = 0 OR EXISTS (
        SELECT 1 FROM questions qq
        WHERE qq.quiz_id = quizzes.id AND qq.category = ANY(sqlc.arg('categories')::text[])
      ))
  AND (sqlc.narg('search')::text IS NULL OR title ILIKE '%' || sqlc.narg('search') || '%' OR description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg('row_limit') OFFSET sqlc.arg('row_offset');

-- name: CountQuizzes :one
SELECT count(*)::bigint AS total
FROM quizzes
WHERE (sqlc.narg('owner_id')::uuid IS NULL OR owner_id = sqlc.narg('owner_id'))
  AND (cardinality(sqlc.arg('statuses')::text[]) = 0 OR status = ANY(sqlc.arg('statuses')::text[]))
  AND (cardinality(sqlc.arg('game_types')::text[]) = 0 OR game_type = ANY(sqlc.arg('game_types')::text[]))
  AND (cardinality(sqlc.arg('categories')::text[]) = 0 OR EXISTS (
        SELECT 1 FROM questions qq
        WHERE qq.quiz_id = quizzes.id AND qq.category = ANY(sqlc.arg('categories')::text[])
      ))
  AND (sqlc.narg('search')::text IS NULL OR title ILIKE '%' || sqlc.narg('search') || '%' OR description ILIKE '%' || sqlc.narg('search') || '%');

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
SELECT q.id, q.title, q.description, q.owner_id, q.game_type, q.created_at,
       (SELECT COUNT(*) FROM questions WHERE quiz_id = q.id)::bigint AS questions_count
FROM quizzes q
WHERE q.status = 'published'
  AND (cardinality(sqlc.arg('game_types')::text[]) = 0 OR q.game_type = ANY(sqlc.arg('game_types')::text[]))
  AND (cardinality(sqlc.arg('categories')::text[]) = 0 OR EXISTS (
        SELECT 1 FROM questions qq
        WHERE qq.quiz_id = q.id AND qq.category = ANY(sqlc.arg('categories')::text[])
      ))
  AND (sqlc.narg('search')::text IS NULL OR q.title ILIKE '%' || sqlc.narg('search') || '%' OR q.description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY q.created_at DESC
LIMIT sqlc.arg('row_limit') OFFSET sqlc.arg('row_offset');

-- name: CountPublicGames :one
SELECT COUNT(*)::bigint
FROM quizzes
WHERE status = 'published'
  AND (cardinality(sqlc.arg('game_types')::text[]) = 0 OR game_type = ANY(sqlc.arg('game_types')::text[]))
  AND (cardinality(sqlc.arg('categories')::text[]) = 0 OR EXISTS (
        SELECT 1 FROM questions qq
        WHERE qq.quiz_id = quizzes.id AND qq.category = ANY(sqlc.arg('categories')::text[])
      ))
  AND (sqlc.narg('search')::text IS NULL OR title ILIKE '%' || sqlc.narg('search') || '%' OR description ILIKE '%' || sqlc.narg('search') || '%');
