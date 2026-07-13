DROP INDEX IF EXISTS exam_set_lifecycle_stop_events_pending_idx;

ALTER TABLE exam_set_lifecycle_stop_events
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS delivery_attempts,
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS claim_token;

CREATE INDEX exam_set_lifecycle_stop_events_pending_idx
ON exam_set_lifecycle_stop_events (exam_set_id, stopped_at)
WHERE delivered_at IS NULL;

DROP TABLE IF EXISTS leaderboard_attempt_projection_outbox;
