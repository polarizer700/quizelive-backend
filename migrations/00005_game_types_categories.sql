-- +goose Up
ALTER TABLE quizzes
    ADD COLUMN game_type text NOT NULL DEFAULT 'quiz'
        CONSTRAINT quizzes_game_type_check CHECK (game_type IN ('quiz', 'pub_quiz', 'jeopardy')),
    ADD COLUMN break_after_round integer NOT NULL DEFAULT 0
        CONSTRAINT quizzes_break_after_round_check CHECK (break_after_round BETWEEN 0 AND 10);

ALTER TABLE questions
    ADD COLUMN category text NOT NULL DEFAULT ''
        CONSTRAINT questions_category_len CHECK (char_length(category) <= 100);

-- +goose Down
ALTER TABLE questions DROP COLUMN category;
ALTER TABLE quizzes DROP COLUMN break_after_round;
ALTER TABLE quizzes DROP COLUMN game_type;
