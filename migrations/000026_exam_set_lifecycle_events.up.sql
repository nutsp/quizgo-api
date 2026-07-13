ALTER TABLE exam_set_lifecycle_stop_events
    RENAME TO exam_set_lifecycle_events;

ALTER TABLE exam_set_lifecycle_events
    RENAME COLUMN stopped_at TO event_at;

ALTER TABLE exam_set_lifecycle_events
    ADD COLUMN event_type varchar(20) NOT NULL DEFAULT 'stopped',
    ADD COLUMN exam_track_id uuid REFERENCES exam_tracks(id);

ALTER TABLE exam_set_lifecycle_events
    DROP CONSTRAINT exam_set_lifecycle_stop_events_pkey,
    ADD CONSTRAINT exam_set_lifecycle_events_pkey PRIMARY KEY (exam_set_id, event_type, event_at),
    ADD CONSTRAINT exam_set_lifecycle_events_type_check CHECK (event_type IN ('published', 'stopped'));

ALTER TABLE exam_set_lifecycle_events
    ALTER COLUMN event_type DROP DEFAULT;

ALTER INDEX exam_set_lifecycle_stop_events_pending_idx
    RENAME TO exam_set_lifecycle_events_pending_idx;
