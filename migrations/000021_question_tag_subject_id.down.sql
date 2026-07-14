ALTER TABLE question_tags DROP CONSTRAINT IF EXISTS fk_question_tags_subject;
DROP INDEX IF EXISTS idx_question_tags_subject_id;
ALTER TABLE question_tags DROP COLUMN IF EXISTS subject_id;
