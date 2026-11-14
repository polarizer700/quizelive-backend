-- +goose Up
ALTER TABLE questions
    ADD COLUMN image_key text,
    ADD COLUMN image_content_type text,
    ADD COLUMN image_size_bytes bigint;

ALTER TABLE questions
    ADD CONSTRAINT questions_image_key_check
        CHECK (image_key IS NULL OR char_length(image_key) BETWEEN 1 AND 500),
    ADD CONSTRAINT questions_image_content_type_check
        CHECK (
            image_content_type IS NULL
            OR image_content_type IN ('image/jpeg', 'image/png', 'image/webp')
        ),
    ADD CONSTRAINT questions_image_size_check
        CHECK (image_size_bytes IS NULL OR (image_size_bytes > 0 AND image_size_bytes <= 5242880)),
    ADD CONSTRAINT questions_image_pair_check
        CHECK (
            (image_key IS NULL AND image_content_type IS NULL AND image_size_bytes IS NULL)
            OR (image_key IS NOT NULL AND image_content_type IS NOT NULL AND image_size_bytes IS NOT NULL)
        );

-- +goose Down
ALTER TABLE questions
    DROP CONSTRAINT questions_image_pair_check,
    DROP CONSTRAINT questions_image_size_check,
    DROP CONSTRAINT questions_image_content_type_check,
    DROP CONSTRAINT questions_image_key_check;

ALTER TABLE questions
    DROP COLUMN image_size_bytes,
    DROP COLUMN image_content_type,
    DROP COLUMN image_key;
