ALTER TABLE exam_attempts DROP COLUMN IF EXISTS blueprint_version;

ALTER TABLE questions
  DROP CONSTRAINT IF EXISTS ck_questions_review_status,
  DROP COLUMN IF EXISTS reviewed_at,
  DROP COLUMN IF EXISTS review_status;

ALTER TABLE exam_tracks
  DROP CONSTRAINT IF EXISTS ck_exam_tracks_blueprint_values,
  DROP CONSTRAINT IF EXISTS ck_exam_tracks_blueprint_status,
  DROP COLUMN IF EXISTS blueprint_sections,
  DROP COLUMN IF EXISTS blueprint_source_note,
  DROP COLUMN IF EXISTS blueprint_reviewed_at,
  DROP COLUMN IF EXISTS blueprint_effective_date,
  DROP COLUMN IF EXISTS blueprint_passing_score,
  DROP COLUMN IF EXISTS blueprint_duration_minutes,
  DROP COLUMN IF EXISTS blueprint_question_count,
  DROP COLUMN IF EXISTS blueprint_status,
  DROP COLUMN IF EXISTS blueprint_version;

