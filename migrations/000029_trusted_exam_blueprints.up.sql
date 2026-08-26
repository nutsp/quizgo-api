ALTER TABLE exam_tracks
  ADD COLUMN blueprint_version INT NOT NULL DEFAULT 1,
  ADD COLUMN blueprint_status VARCHAR(20) NOT NULL DEFAULT 'draft',
  ADD COLUMN blueprint_question_count INT NOT NULL DEFAULT 0,
  ADD COLUMN blueprint_duration_minutes INT NOT NULL DEFAULT 0,
  ADD COLUMN blueprint_passing_score INT NOT NULL DEFAULT 0,
  ADD COLUMN blueprint_effective_date DATE,
  ADD COLUMN blueprint_reviewed_at TIMESTAMPTZ,
  ADD COLUMN blueprint_source_note TEXT NOT NULL DEFAULT '',
  ADD COLUMN blueprint_sections JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE questions
  ADD COLUMN review_status VARCHAR(20) NOT NULL DEFAULT 'unreviewed',
  ADD COLUMN reviewed_at TIMESTAMPTZ;

UPDATE questions
SET review_status = 'reviewed', reviewed_at = COALESCE(updated_at, created_at)
WHERE status = 'published';

ALTER TABLE exam_attempts
  ADD COLUMN blueprint_version INT NOT NULL DEFAULT 1;

ALTER TABLE exam_tracks
  ADD CONSTRAINT ck_exam_tracks_blueprint_status
    CHECK (blueprint_status IN ('draft', 'reviewed')),
  ADD CONSTRAINT ck_exam_tracks_blueprint_values
    CHECK (
      blueprint_question_count >= 0
      AND blueprint_duration_minutes >= 0
      AND blueprint_passing_score BETWEEN 0 AND 100
    );

ALTER TABLE questions
  ADD CONSTRAINT ck_questions_review_status
    CHECK (review_status IN ('unreviewed', 'reviewed', 'needs_review'));

