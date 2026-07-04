CREATE INDEX IF NOT EXISTS idx_question_import_rows_job_row_number
ON question_import_rows(import_job_id, row_number);

CREATE INDEX IF NOT EXISTS idx_question_import_rows_job_valid
ON question_import_rows(import_job_id, valid);
