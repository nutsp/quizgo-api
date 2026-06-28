CREATE TABLE question_tags (
  id UUID PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  code VARCHAR(100) NOT NULL UNIQUE,
  description TEXT NULL,
  color VARCHAR(20) NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE question_tag_mappings (
  id UUID PRIMARY KEY,
  question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  tag_id UUID NOT NULL REFERENCES question_tags(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  UNIQUE(question_id, tag_id)
);

CREATE INDEX idx_question_tag_mappings_question_id ON question_tag_mappings(question_id);
CREATE INDEX idx_question_tag_mappings_tag_id ON question_tag_mappings(tag_id);

ALTER TABLE question_import_rows ADD COLUMN IF NOT EXISTS tags VARCHAR(500) NULL;
