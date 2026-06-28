ALTER TABLE question_import_rows DROP COLUMN IF EXISTS tags;

DROP INDEX IF EXISTS idx_question_tag_mappings_tag_id;
DROP INDEX IF EXISTS idx_question_tag_mappings_question_id;
DROP TABLE IF EXISTS question_tag_mappings;
DROP TABLE IF EXISTS question_tags;
