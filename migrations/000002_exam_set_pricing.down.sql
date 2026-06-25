DROP INDEX IF EXISTS idx_exam_sets_featured;

ALTER TABLE exam_sets
    DROP COLUMN IF EXISTS is_featured,
    DROP COLUMN IF EXISTS sale_price_amount,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS price_amount,
    DROP COLUMN IF EXISTS cover_image_url;
