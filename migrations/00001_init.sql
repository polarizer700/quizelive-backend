-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    password_hash text NOT NULL,
    role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_email_normalized CHECK (email = lower(email) AND char_length(email) BETWEEN 3 AND 320),
    CONSTRAINT users_role_check CHECK (role IN ('admin', 'editor', 'host'))
);

CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CONSTRAINT refresh_tokens_hash_key UNIQUE (token_hash)
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

CREATE TABLE quizzes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    owner_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'draft',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT quizzes_title_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT quizzes_description_check CHECK (char_length(description) <= 2000),
    CONSTRAINT quizzes_status_check CHECK (status IN ('draft', 'published'))
);

CREATE INDEX quizzes_owner_id_idx ON quizzes (owner_id);

CREATE TABLE questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id uuid NOT NULL REFERENCES quizzes (id) ON DELETE CASCADE,
    position integer NOT NULL,
    type text NOT NULL,
    text text NOT NULL,
    time_limit_sec integer NOT NULL,
    points integer NOT NULL,
    CONSTRAINT questions_quiz_position_key UNIQUE (quiz_id, position),
    CONSTRAINT questions_position_check CHECK (position <> 0),
    CONSTRAINT questions_type_check CHECK (type IN ('single', 'multiple', 'text')),
    CONSTRAINT questions_text_check CHECK (char_length(text) BETWEEN 1 AND 2000),
    CONSTRAINT questions_time_limit_check CHECK (time_limit_sec BETWEEN 5 AND 300),
    CONSTRAINT questions_points_check CHECK (points BETWEEN 1 AND 10000)
);

CREATE INDEX questions_quiz_id_idx ON questions (quiz_id);

CREATE TABLE answer_options (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id uuid NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    position integer NOT NULL,
    text text NOT NULL,
    is_correct boolean NOT NULL DEFAULT false,
    CONSTRAINT answer_options_question_position_key UNIQUE (question_id, position),
    CONSTRAINT answer_options_position_check CHECK (position > 0),
    CONSTRAINT answer_options_text_check CHECK (char_length(text) BETWEEN 1 AND 500)
);

CREATE INDEX answer_options_question_id_idx ON answer_options (question_id);

CREATE TABLE text_answers (
    question_id uuid NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    normalized_value text NOT NULL,
    PRIMARY KEY (question_id, normalized_value),
    CONSTRAINT text_answers_value_check CHECK (char_length(normalized_value) BETWEEN 1 AND 200)
);

CREATE TABLE game_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id uuid NOT NULL REFERENCES quizzes (id) ON DELETE RESTRICT,
    host_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    code text NOT NULL,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT game_sessions_code_key UNIQUE (code)
);

CREATE TABLE game_results (
    session_id uuid NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    nickname text NOT NULL,
    score integer NOT NULL,
    rank integer NOT NULL,
    PRIMARY KEY (session_id, nickname)
);

-- +goose Down
DROP TABLE IF EXISTS game_results;
DROP TABLE IF EXISTS game_sessions;
DROP TABLE IF EXISTS text_answers;
DROP TABLE IF EXISTS answer_options;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS quizzes;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
