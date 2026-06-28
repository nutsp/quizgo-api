ALTER TABLE exam_sets
DROP CONSTRAINT IF EXISTS exam_sets_access_type_check;

ALTER TABLE exam_sets
ADD CONSTRAINT exam_sets_access_type_check
CHECK (access_type IN ('free', 'paid', 'premium'));

ALTER TABLE exam_sets
DROP COLUMN IF EXISTS allow_single_purchase;
