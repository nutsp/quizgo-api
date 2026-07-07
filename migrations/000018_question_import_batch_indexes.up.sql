CREATE INDEX IF NOT EXISTS idx_question_import_jobs_status
ON question_import_jobs(status);

CREATE INDEX IF NOT EXISTS idx_question_import_jobs_created_at
ON question_import_jobs(created_at);

CREATE INDEX IF NOT EXISTS idx_question_import_jobs_admin_user_id
ON question_import_jobs(admin_user_id);

CREATE INDEX IF NOT EXISTS idx_question_import_rows_job_valid_batch
ON question_import_rows(import_job_id, valid);
