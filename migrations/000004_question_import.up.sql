CREATE TABLE question_import_jobs (
    id UUID PRIMARY KEY,
    admin_user_id UUID NOT NULL REFERENCES users(id),
    filename TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'preview',
    total_rows INT NOT NULL DEFAULT 0,
    valid_rows INT NOT NULL DEFAULT 0,
    invalid_rows INT NOT NULL DEFAULT 0,
    imported_questions INT NOT NULL DEFAULT 0,
    skipped_rows INT NOT NULL DEFAULT 0,
    failed_rows INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMP NULL
);

CREATE TABLE question_import_rows (
    id UUID PRIMARY KEY,
    import_job_id UUID NOT NULL REFERENCES question_import_jobs(id) ON DELETE CASCADE,
    row_number INT NOT NULL,
    subject_code VARCHAR(100),
    question_text TEXT,
    choice_a TEXT,
    choice_b TEXT,
    choice_c TEXT,
    choice_d TEXT,
    correct_choice VARCHAR(10),
    explanation TEXT,
    difficulty VARCHAR(50),
    status VARCHAR(50),
    valid BOOLEAN NOT NULL DEFAULT false,
    errors JSONB NOT NULL DEFAULT '[]',
    warnings JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_question_import_rows_job_id ON question_import_rows(import_job_id);
