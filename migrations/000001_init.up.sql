-- 000001_init.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TABLE IF NOT EXISTS exam_tracks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    cover_image_url TEXT,
    total_exam_sets INT NOT NULL DEFAULT 0,
    total_questions INT NOT NULL DEFAULT 0,
    total_attempts INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_tracks_code UNIQUE (code)
);

CREATE TABLE IF NOT EXISTS exam_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_track_id UUID NOT NULL REFERENCES exam_tracks(id),
    code VARCHAR(100) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    cover_image_url TEXT,
    duration_minutes INT NOT NULL,
    total_questions INT NOT NULL,
    passing_score INT NOT NULL,
    difficulty VARCHAR(50) NOT NULL,
    access_type VARCHAR(50) NOT NULL,
    price_amount NUMERIC NOT NULL DEFAULT 0,
    currency VARCHAR(10) NOT NULL DEFAULT 'THB',
    sale_price_amount NUMERIC,
    mode VARCHAR(50) NOT NULL,
    is_official BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_sets_code UNIQUE (code)
);

CREATE INDEX IF NOT EXISTS idx_exam_sets_track ON exam_sets(exam_track_id);
CREATE INDEX IF NOT EXISTS idx_exam_sets_active ON exam_sets(is_active);

CREATE TABLE IF NOT EXISTS subjects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_subjects_code UNIQUE (code)
);

CREATE TABLE IF NOT EXISTS questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES subjects(id),
    question_text TEXT NOT NULL,
    explanation TEXT,
    difficulty VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_questions_subject ON questions(subject_id);

CREATE TABLE IF NOT EXISTS choices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    choice_key VARCHAR(1) NOT NULL,
    choice_label VARCHAR(10) NOT NULL,
    choice_text TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_choices_question ON choices(question_id);

CREATE TABLE IF NOT EXISTS exam_set_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_set_id UUID NOT NULL REFERENCES exam_sets(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id),
    question_no INT NOT NULL,
    score NUMERIC(10,2) NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_set_no UNIQUE (exam_set_id, question_no),
    CONSTRAINT uq_exam_set_question UNIQUE (exam_set_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_exam_set_questions_set ON exam_set_questions(exam_set_id);

CREATE TABLE IF NOT EXISTS exam_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    exam_track_id UUID NOT NULL REFERENCES exam_tracks(id),
    exam_set_id UUID NOT NULL REFERENCES exam_sets(id),
    status VARCHAR(50) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    duration_seconds INT,
    score NUMERIC(10,2) NOT NULL DEFAULT 0,
    total_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    score_percent NUMERIC(10,2) NOT NULL DEFAULT 0,
    correct_count INT NOT NULL DEFAULT 0,
    wrong_count INT NOT NULL DEFAULT 0,
    unanswered_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_exam_attempts_user ON exam_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_status ON exam_attempts(status);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_set ON exam_attempts(exam_set_id);

CREATE TABLE IF NOT EXISTS exam_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id UUID NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id),
    question_no INT NOT NULL,
    selected_choice_key VARCHAR(1),
    is_correct BOOLEAN,
    answered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_attempt_question UNIQUE (attempt_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_exam_answers_attempt ON exam_answers(attempt_id);
