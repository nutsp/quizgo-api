DELETE FROM exam_set_lifecycle_events WHERE event_type = 'published';

ALTER TABLE exam_set_lifecycle_events
    DROP CONSTRAINT exam_set_lifecycle_events_pkey,
    DROP CONSTRAINT exam_set_lifecycle_events_type_check,
    DROP COLUMN exam_track_id,
    DROP COLUMN event_type;

ALTER TABLE exam_set_lifecycle_events
    ADD CONSTRAINT exam_set_lifecycle_stop_events_pkey PRIMARY KEY (exam_set_id, event_at);

ALTER TABLE exam_set_lifecycle_events
    RENAME COLUMN event_at TO stopped_at;

ALTER TABLE exam_set_lifecycle_events
    RENAME TO exam_set_lifecycle_stop_events;

ALTER INDEX exam_set_lifecycle_events_pending_idx
    RENAME TO exam_set_lifecycle_stop_events_pending_idx;
