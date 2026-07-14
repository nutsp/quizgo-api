ALTER TABLE exam_attempts
ADD COLUMN IF NOT EXISTS timing_mode VARCHAR(20) NOT NULL DEFAULT 'countdown';

ALTER TABLE exam_attempts
DROP CONSTRAINT IF EXISTS exam_attempts_timing_mode_check;

ALTER TABLE exam_attempts
ADD CONSTRAINT exam_attempts_timing_mode_check
CHECK (timing_mode IN ('countdown', 'elapsed'));
