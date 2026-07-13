CREATE TABLE IF NOT EXISTS exam_set_lifecycle_stop_events (
    exam_set_id uuid NOT NULL,
    stopped_at timestamptz NOT NULL,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (exam_set_id, stopped_at)
);

CREATE INDEX IF NOT EXISTS exam_set_lifecycle_stop_events_pending_idx
ON exam_set_lifecycle_stop_events (exam_set_id, stopped_at)
WHERE delivered_at IS NULL;
