CREATE TABLE exam_set_question_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  exam_set_id UUID NOT NULL REFERENCES exam_sets(id) ON DELETE CASCADE,
  rule_order INT NOT NULL DEFAULT 1,
  subject_id UUID NOT NULL REFERENCES subjects(id),
  tag_id UUID NULL REFERENCES question_tags(id) ON DELETE SET NULL,
  difficulty VARCHAR(20) NULL,
  count INT NOT NULL CHECK (count > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exam_set_question_rules_exam_set ON exam_set_question_rules(exam_set_id);
CREATE UNIQUE INDEX idx_exam_set_question_rules_order ON exam_set_question_rules(exam_set_id, rule_order);
