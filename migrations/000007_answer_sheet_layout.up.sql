-- 000007_answer_sheet_layout.up.sql
ALTER TABLE exam_sets
ADD COLUMN IF NOT EXISTS answer_sheet_block_columns INT NOT NULL DEFAULT 2,
ADD COLUMN IF NOT EXISTS answer_sheet_questions_per_block INT NOT NULL DEFAULT 10,
ADD COLUMN IF NOT EXISTS answer_sheet_choice_label_style VARCHAR(20) NOT NULL DEFAULT 'thai',
ADD COLUMN IF NOT EXISTS answer_sheet_show_header BOOLEAN NOT NULL DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS answer_sheet_show_instructions BOOLEAN NOT NULL DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS answer_sheet_show_candidate_info BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE exam_sets
ADD CONSTRAINT chk_answer_sheet_block_columns
  CHECK (answer_sheet_block_columns >= 1 AND answer_sheet_block_columns <= 4);

ALTER TABLE exam_sets
ADD CONSTRAINT chk_answer_sheet_questions_per_block
  CHECK (answer_sheet_questions_per_block >= 5 AND answer_sheet_questions_per_block <= 50);

ALTER TABLE exam_sets
ADD CONSTRAINT chk_answer_sheet_choice_label_style
  CHECK (answer_sheet_choice_label_style IN ('thai', 'english'));
