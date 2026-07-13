CREATE TABLE leaderboard_attempt_projection_outbox (
    attempt_id uuid PRIMARY KEY REFERENCES exam_attempts(id),
    user_id uuid NOT NULL REFERENCES users(id),
    exam_set_id uuid NOT NULL REFERENCES exam_sets(id),
    exam_track_id uuid NOT NULL REFERENCES exam_tracks(id),
    track_code text NOT NULL,
    submitted_at timestamptz NOT NULL,
    points numeric(6,1) NOT NULL,
    duration_seconds int NOT NULL,
    delivered_at timestamptz,
    claim_token uuid,
    claimed_at timestamptz,
    delivery_attempts int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX leaderboard_attempt_projection_outbox_pending_idx
ON leaderboard_attempt_projection_outbox (next_attempt_at, created_at)
WHERE delivered_at IS NULL;

ALTER TABLE exam_set_lifecycle_stop_events
    ADD COLUMN claim_token uuid,
    ADD COLUMN claimed_at timestamptz,
    ADD COLUMN delivery_attempts int NOT NULL DEFAULT 0,
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN last_error text;

DROP INDEX IF EXISTS exam_set_lifecycle_stop_events_pending_idx;
CREATE INDEX exam_set_lifecycle_stop_events_pending_idx
ON exam_set_lifecycle_stop_events (next_attempt_at, created_at)
WHERE delivered_at IS NULL;
