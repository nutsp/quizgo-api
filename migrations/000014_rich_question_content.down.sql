ALTER TABLE question_import_rows
    DROP COLUMN IF EXISTS choice_d_image_url,
    DROP COLUMN IF EXISTS choice_d_image,
    DROP COLUMN IF EXISTS choice_c_image_url,
    DROP COLUMN IF EXISTS choice_c_image,
    DROP COLUMN IF EXISTS choice_b_image_url,
    DROP COLUMN IF EXISTS choice_b_image,
    DROP COLUMN IF EXISTS choice_a_image_url,
    DROP COLUMN IF EXISTS choice_a_image,
    DROP COLUMN IF EXISTS explanation_image_url,
    DROP COLUMN IF EXISTS explanation_image,
    DROP COLUMN IF EXISTS question_image_url,
    DROP COLUMN IF EXISTS question_image,
    DROP COLUMN IF EXISTS content_format,
    DROP COLUMN IF EXISTS question_type;

ALTER TABLE choices
    DROP COLUMN IF EXISTS choice_image_url,
    DROP COLUMN IF EXISTS content_format;

ALTER TABLE questions
    DROP COLUMN IF EXISTS explanation_image_url,
    DROP COLUMN IF EXISTS question_image_url,
    DROP COLUMN IF EXISTS content_format;
