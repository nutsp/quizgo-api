ALTER TABLE exam_sets DROP CONSTRAINT IF EXISTS chk_answer_sheet_choice_label_style;
ALTER TABLE exam_sets DROP CONSTRAINT IF EXISTS chk_answer_sheet_questions_per_block;
ALTER TABLE exam_sets DROP CONSTRAINT IF EXISTS chk_answer_sheet_block_columns;

ALTER TABLE exam_sets
DROP COLUMN IF EXISTS answer_sheet_show_candidate_info,
DROP COLUMN IF EXISTS answer_sheet_show_instructions,
DROP COLUMN IF EXISTS answer_sheet_show_header,
DROP COLUMN IF EXISTS answer_sheet_choice_label_style,
DROP COLUMN IF EXISTS answer_sheet_questions_per_block,
DROP COLUMN IF EXISTS answer_sheet_block_columns;
