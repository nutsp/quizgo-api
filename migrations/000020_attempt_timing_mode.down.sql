ALTER TABLE exam_attempts
DROP CONSTRAINT IF EXISTS exam_attempts_timing_mode_check;

ALTER TABLE exam_attempts
DROP COLUMN IF EXISTS timing_mode;
