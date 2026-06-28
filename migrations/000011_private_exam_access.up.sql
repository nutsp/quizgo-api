ALTER TABLE exam_sets
DROP CONSTRAINT IF EXISTS exam_sets_access_type_check;

ALTER TABLE exam_sets
ADD CONSTRAINT exam_sets_access_type_check
CHECK (access_type IN ('free', 'paid', 'premium', 'private'));

ALTER TABLE exam_sets
ADD COLUMN IF NOT EXISTS allow_single_purchase BOOLEAN NOT NULL DEFAULT false;
