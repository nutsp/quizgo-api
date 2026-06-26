ALTER TABLE exam_sets
ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'draft';

-- Existing active sets with questions were live before status existed; treat as published.
UPDATE exam_sets
SET status = 'published'
WHERE is_active = true
  AND total_questions > 0;

CREATE INDEX IF NOT EXISTS idx_exam_sets_status ON exam_sets (status);
