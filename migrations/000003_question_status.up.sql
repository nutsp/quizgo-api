ALTER TABLE questions
ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'draft';

ALTER TABLE questions
ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

UPDATE questions SET status = 'published' WHERE status = 'draft';
