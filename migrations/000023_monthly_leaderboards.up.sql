CREATE TABLE leaderboard_seasons (
    id uuid PRIMARY KEY,
    exam_track_id uuid NOT NULL REFERENCES exam_tracks(id),
    year int NOT NULL,
    month int NOT NULL CHECK (month BETWEEN 1 AND 12),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    status varchar(20) NOT NULL CHECK (status IN ('active', 'finalized')),
    finalized_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (exam_track_id, year, month)
);

CREATE TABLE leaderboard_season_exam_sets (
    id uuid PRIMARY KEY,
    season_id uuid NOT NULL REFERENCES leaderboard_seasons(id) ON DELETE CASCADE,
    exam_set_id uuid NOT NULL REFERENCES exam_sets(id),
    joined_at timestamptz NOT NULL,
    stopped_at timestamptz,
    CONSTRAINT leaderboard_season_exam_sets_interval_key
        UNIQUE (season_id, exam_set_id, joined_at)
);

CREATE UNIQUE INDEX leaderboard_season_exam_sets_one_open_idx
ON leaderboard_season_exam_sets (season_id, exam_set_id)
WHERE stopped_at IS NULL;

CREATE TABLE leaderboard_scores (
    season_id uuid NOT NULL REFERENCES leaderboard_seasons(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    exam_set_id uuid NOT NULL REFERENCES exam_sets(id),
    attempt_id uuid NOT NULL REFERENCES exam_attempts(id),
    points numeric(6,1) NOT NULL,
    duration_seconds int NOT NULL,
    achieved_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (season_id, user_id, exam_set_id)
);

CREATE TABLE leaderboard_entries (
    season_id uuid NOT NULL REFERENCES leaderboard_seasons(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    total_points numeric(10,1) NOT NULL,
    completed_exam_sets int NOT NULL,
    total_duration_seconds bigint NOT NULL,
    score_achieved_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (season_id, user_id)
);

CREATE INDEX leaderboard_entries_rank_idx
ON leaderboard_entries (
    season_id, total_points DESC, completed_exam_sets DESC,
    total_duration_seconds ASC, score_achieved_at ASC
);

CREATE TABLE leaderboard_awards (
    id uuid PRIMARY KEY,
    season_id uuid NOT NULL REFERENCES leaderboard_seasons(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    rank int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (season_id, user_id, rank)
);

CREATE TABLE leaderboard_projection_failures (
    id uuid PRIMARY KEY,
    attempt_id uuid NOT NULL REFERENCES exam_attempts(id),
    retry_count int NOT NULL DEFAULT 0,
    last_error text NOT NULL,
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (attempt_id)
);
