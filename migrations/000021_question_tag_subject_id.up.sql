ALTER TABLE question_tags ADD COLUMN IF NOT EXISTS subject_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_question_tags_subject_id ON question_tags(subject_id);

ALTER TABLE question_tags
  ADD CONSTRAINT fk_question_tags_subject
  FOREIGN KEY (subject_id) REFERENCES subjects(id) ON DELETE SET NULL;
