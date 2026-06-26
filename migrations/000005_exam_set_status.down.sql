DROP INDEX IF EXISTS idx_exam_sets_status;
ALTER TABLE exam_sets DROP COLUMN IF EXISTS status;
