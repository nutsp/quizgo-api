CREATE TABLE announcements (
    id uuid PRIMARY KEY,
    title varchar(255) NOT NULL,
    slug varchar(255) NOT NULL,
    summary text,
    content text,
    type varchar(30) NOT NULL,
    priority integer NOT NULL DEFAULT 0,
    is_pinned boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    publish_status varchar(20) NOT NULL DEFAULT 'draft',
    starts_at timestamptz,
    ends_at timestamptz,
    exam_track_id uuid REFERENCES exam_tracks(id),
    exam_date date,
    days_before_start integer NOT NULL DEFAULT 0,
    cta_label varchar(255),
    cta_url text,
    created_by uuid REFERENCES users(id),
    updated_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT announcements_type_check CHECK (
        type IN ('general', 'exam_schedule', 'exam_update', 'promotion', 'maintenance', 'system')
    ),
    CONSTRAINT announcements_publish_status_check CHECK (
        publish_status IN ('draft', 'published', 'archived')
    ),
    CONSTRAINT announcements_days_before_start_check CHECK (days_before_start >= 0),
    CONSTRAINT announcements_display_window_check CHECK (
        starts_at IS NULL OR ends_at IS NULL OR starts_at < ends_at
    ),
    CONSTRAINT announcements_exam_schedule_check CHECK (
        type <> 'exam_schedule' OR (exam_track_id IS NOT NULL AND exam_date IS NOT NULL)
    )
);

CREATE UNIQUE INDEX announcements_slug_lower_uidx ON announcements (LOWER(slug));
CREATE INDEX announcements_public_visibility_idx
    ON announcements (publish_status, is_active, is_pinned DESC, priority DESC, updated_at DESC);
CREATE INDEX announcements_exam_track_idx ON announcements (exam_track_id);
CREATE INDEX announcements_exam_date_idx ON announcements (exam_date) WHERE type = 'exam_schedule';

CREATE TABLE announcement_exam_sets (
    announcement_id uuid NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    exam_set_id uuid NOT NULL REFERENCES exam_sets(id),
    sort_order integer NOT NULL DEFAULT 0,
    PRIMARY KEY (announcement_id, exam_set_id),
    CONSTRAINT announcement_exam_sets_order_uidx UNIQUE (announcement_id, sort_order)
);

CREATE INDEX announcement_exam_sets_exam_set_idx ON announcement_exam_sets (exam_set_id);
