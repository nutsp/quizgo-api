ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS content_format VARCHAR(30) NOT NULL DEFAULT 'plain',
    ADD COLUMN IF NOT EXISTS question_image_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS explanation_image_url TEXT NULL;

ALTER TABLE choices
    ADD COLUMN IF NOT EXISTS content_format VARCHAR(30) NOT NULL DEFAULT 'plain',
    ADD COLUMN IF NOT EXISTS choice_image_url TEXT NULL;

ALTER TABLE question_import_rows
    ADD COLUMN IF NOT EXISTS question_type VARCHAR(30) NULL,
    ADD COLUMN IF NOT EXISTS content_format VARCHAR(30) NULL,
    ADD COLUMN IF NOT EXISTS question_image TEXT NULL,
    ADD COLUMN IF NOT EXISTS question_image_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS explanation_image TEXT NULL,
    ADD COLUMN IF NOT EXISTS explanation_image_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_a_image TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_a_image_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_b_image TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_b_image_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_c_image TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_c_image_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_d_image TEXT NULL,
    ADD COLUMN IF NOT EXISTS choice_d_image_url TEXT NULL;
