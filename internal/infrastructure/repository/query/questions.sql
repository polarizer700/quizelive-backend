-- name: ListQuestionCategories :many
SELECT DISTINCT category
FROM questions
WHERE category <> ''
  AND (sqlc.narg('search')::text IS NULL OR category ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY category
LIMIT sqlc.arg('row_limit');

-- name: CreateQuestion :one
INSERT INTO questions (quiz_id, position, type, text, time_limit_sec, points, category)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, quiz_id, position, type, text, time_limit_sec, points, category;

-- name: NextQuestionPosition :one
SELECT (COALESCE(MAX(position), 0) + 1)::int AS position
FROM questions
WHERE quiz_id = $1
  AND position > 0;

-- name: GetQuestion :one
SELECT id, quiz_id, position, type, text, time_limit_sec, points, category
FROM questions
WHERE id = $1
  AND quiz_id = $2;

-- name: ListQuestionsByQuiz :many
SELECT id, quiz_id, position, type, text, time_limit_sec, points, category
FROM questions
WHERE quiz_id = $1
  AND position > 0
ORDER BY position;

-- name: UpdateQuestion :one
UPDATE questions
SET type = $2,
    text = $3,
    time_limit_sec = $4,
    points = $5,
    category = $6
WHERE id = $1
RETURNING id, quiz_id, position, type, text, time_limit_sec, points, category;

-- name: DeleteQuestion :exec
DELETE FROM questions
WHERE id = $1
  AND quiz_id = $2;

-- name: NegateQuestionPositions :exec
UPDATE questions
SET position = -position
WHERE quiz_id = $1
  AND position > 0;

-- name: SetQuestionPosition :exec
UPDATE questions
SET position = $3
WHERE quiz_id = $1
  AND id = $2;

-- name: DeleteOptionsByQuestion :exec
DELETE FROM answer_options
WHERE question_id = $1;

-- name: CreateOption :one
INSERT INTO answer_options (question_id, position, text, is_correct)
VALUES ($1, $2, $3, $4)
RETURNING id, question_id, position, text, is_correct;

-- name: ListOptionsByQuiz :many
SELECT o.id, o.question_id, o.position, o.text, o.is_correct
FROM answer_options o
JOIN questions q ON q.id = o.question_id
WHERE q.quiz_id = $1
ORDER BY o.question_id, o.position;

-- name: DeleteTextAnswersByQuestion :exec
DELETE FROM text_answers
WHERE question_id = $1;

-- name: CreateTextAnswer :exec
INSERT INTO text_answers (question_id, normalized_value)
VALUES ($1, $2);

-- name: ListTextAnswersByQuiz :many
SELECT t.question_id, t.normalized_value
FROM text_answers t
JOIN questions q ON q.id = t.question_id
WHERE q.quiz_id = $1
ORDER BY t.question_id, t.normalized_value;
