# Announcement Feature with Exam Schedule Alerts Design

**Date:** 2026-07-14

**Status:** Approved for implementation planning

## Goal

Add an announcement system for QuizGo that lets administrators manage general and exam-related announcements while public users see only currently visible published content. Exam schedule announcements count down to an exam date and direct users to the related exam track or recommended exam sets.

The MVP intentionally excludes notification bells and read/unread state.

## Approved Product Decisions

- Announcement types are `general`, `exam_schedule`, `exam_update`, `promotion`, `maintenance`, and `system`.
- Publish statuses are `draft`, `published`, and `archived`.
- For `exam_schedule`, `exam_track_id` and `exam_date` are required.
- When `starts_at` is null, an exam schedule announcement becomes visible on `exam_date - days_before_start`.
- The Admin form does not expose `priority`, `starts_at`, or `ends_at` in the MVP.
- Admin-created announcements therefore persist `priority = 0`, `starts_at = null`, and `ends_at = null` unless another trusted API client supplies supported values later.
- `is_pinned` overrides the past-exam-date rule only. It does not override inactive, unpublished, `starts_at`, or `ends_at` filtering.
- Calendar calculations use the `Asia/Bangkok` timezone.
- Public detail uses the same visibility policy as public list endpoints. An expired, inactive, draft, or archived announcement cannot be opened by slug.
- Deleting an announcement hard deletes the announcement and cascades to its exam-set links.
- Recommended exam-set order follows the incoming `exam_set_ids` array.
- Public responses omit recommended exam sets that are not both published and active.

## Architecture

Implement announcements as an independent vertical slice that follows the existing backend boundaries:

```text
internal/announcement/
  domain/
  repository/
  usecase/
  transport/http/
```

- The handler owns HTTP binding, status codes, and response writing.
- The use case owns validation, visibility, status transitions, countdown calculations, cache interaction, and mutation orchestration.
- The repository owns PostgreSQL queries, persistence, ordering, and model mapping.
- PostgreSQL remains the source of truth.
- The existing audit logger records successful admin mutations.
- The existing content cache and set-index invalidation mechanism cache public reads without Redis `KEYS`.

The frontend adds feature-owned API types and components while reusing the existing Admin full-card/data-list/form primitives. Public announcement presentation is shared by Home, exam-track landing pages, and announcement detail.

## Database Design

### `announcements`

```text
id                  uuid primary key
title               varchar not null
slug                varchar not null
summary             text null
content             text null
type                varchar not null
priority            integer not null default 0
is_pinned           boolean not null default false
is_active           boolean not null default true
publish_status      varchar not null default 'draft'
starts_at           timestamptz null
ends_at             timestamptz null
exam_track_id       uuid null references exam_tracks(id)
exam_date            date null
days_before_start   integer not null default 0
cta_label           varchar null
cta_url             text null
created_by          uuid null references users(id)
updated_by          uuid null references users(id)
created_at          timestamptz not null default now()
updated_at          timestamptz not null default now()
```

Constraints and indexes:

- Case-insensitive unique slug index using `LOWER(slug)`.
- Check constraint for allowed announcement types.
- Check constraint for allowed publish statuses.
- Check constraint `days_before_start >= 0`.
- Check constraint `ends_at IS NULL OR starts_at IS NULL OR starts_at < ends_at`.
- Check constraint requiring `exam_track_id` and `exam_date` when `type = 'exam_schedule'`.
- Public-read indexes covering publish status, active state, display windows, pinning, priority, track, and exam date.

### `announcement_exam_sets`

```text
announcement_id   uuid not null references announcements(id) on delete cascade
exam_set_id       uuid not null references exam_sets(id)
sort_order        integer not null default 0
primary key (announcement_id, exam_set_id)
unique (announcement_id, sort_order)
```

Exam-set replacement during create/update is transactional so an announcement never exposes a partially updated recommendation order.

## API Contract

### Admin

```http
GET    /api/v1/admin/announcements
POST   /api/v1/admin/announcements
GET    /api/v1/admin/announcements/{id}
PATCH  /api/v1/admin/announcements/{id}
PATCH  /api/v1/admin/announcements/{id}/status
DELETE /api/v1/admin/announcements/{id}
```

Create/update accepts the visible Admin form fields and an ordered `exam_set_ids` array. The backend also understands `priority`, `starts_at`, and `ends_at` for forward-compatible trusted API use, but the MVP Admin UI does not render them.

Status request:

```json
{
  "publish_status": "published"
}
```

Status semantics:

- `published` publishes after full validation.
- `draft` unpublishes.
- `archived` archives.
- Any unsupported status returns `ANNOUNCEMENT_INVALID_STATUS`.

### Public

```http
GET /api/v1/announcements/active
GET /api/v1/announcements/active?type=exam_schedule
GET /api/v1/announcements/{slug}
GET /api/v1/exam-tracks/{trackSlug}/announcements
```

The active endpoint returns all currently visible announcements ordered by:

```text
is_pinned DESC, priority DESC, updated_at DESC
```

Home selects at most three announcements where `is_pinned = true` or `priority > 0`. Track pages show all currently visible announcements related to that track.

Public announcement responses include:

- Core announcement content and display metadata.
- Related exam-track summary when present.
- Published and active recommended exam sets ordered by `sort_order`.
- Derived `days_left` for exam schedules.
- Derived display text for exam schedules.

CTA resolution order is:

1. Explicit `cta_url`.
2. Related exam-track page `/exams/{trackSlug}`.

Recommended exam sets appear as additional links and never create an exam attempt automatically.

## Visibility and Time Rules

A public announcement is visible only when all of these are true:

```text
publish_status = published
is_active = true
starts_at is null or starts_at <= now
ends_at is null or ends_at >= now
```

For `exam_schedule`, the effective start is:

```text
starts_at when present
otherwise midnight Asia/Bangkok on exam_date - days_before_start
```

Before that effective start, the announcement is hidden. After `exam_date`, the announcement is hidden unless `is_pinned = true`.

`days_left` is the calendar-day difference between today's Bangkok date and `exam_date`:

- `0`: `สอบวันนี้`
- `1`: `เหลืออีก 1 วัน`
- Greater than `1`: `เหลืออีก {days_left} วัน`

The public API does not return negative `days_left` for an unpinned announcement because that announcement is already hidden. A pinned past schedule may return a negative value for machine-readable accuracy, while the UI omits countdown copy and shows the configured title/summary instead.

## Validation and Error Contract

Validation runs in both the Admin Zod schema and the Go use case. Database constraints protect the persistence boundary.

- `title` is required.
- `slug` is required, lowercase URL-safe, and unique case-insensitively.
- `type` must be allowed.
- `exam_schedule` requires `exam_track_id` and `exam_date`.
- `days_before_start` must be at least zero.
- When both are present, `starts_at` must be before `ends_at`.
- `cta_url` must be either an internal path beginning with one `/` or a valid `http`/`https` URL.
- Referenced exam tracks and exam sets must exist.
- Publishing re-runs full validation.

Feature errors use the existing response envelope and Thai messages:

```text
ANNOUNCEMENT_NOT_FOUND
ANNOUNCEMENT_SLUG_TAKEN
ANNOUNCEMENT_INVALID_STATUS
VALIDATION_ERROR
```

Public lookups return not found for content that fails public visibility checks, preventing disclosure of unpublished records.

## Cache Design

Public reads use these keys:

```text
announcements:active
announcements:active:type:{type}
announcements:track:{trackSlug}
```

Each populated key is added to an announcement cache index set. Create, update, publish, unpublish, archive, and delete invalidate that index with the existing `SMEMBERS` plus pipelined `DEL` pattern. No Redis `KEYS` command is used.

The cache TTL is short enough for schedule boundary recovery even if no mutation occurs. Cached active data is rechecked by the use case when a time boundary can make a cached record stale.

## Audit Logs

Successful admin actions write:

```text
announcement.create
announcement.update
announcement.publish
announcement.unpublish
announcement.archive
announcement.delete
```

Audit records use resource type `announcement`, include the announcement ID and title, and capture sanitized before/after snapshots where applicable. Failed mutations do not write success audit records.

## Admin UI

Add `ประกาศ` to the existing Admin content sidebar and add:

```text
/admin/announcements
/admin/announcements/new
/admin/announcements/{id}/edit
```

The list follows the existing full-card/data-list layout on desktop and mobile. It includes the specified columns, search/status/type filters, loading, error, empty, confirmation, and success states. Row actions support edit, publish/unpublish, archive, and delete.

The form uses the existing Admin form shell and section components. It includes:

- หัวข้อประกาศ
- Slug
- ประเภท
- คำอธิบายสั้น
- เนื้อหา
- รายการสอบ
- ชุดข้อสอบแนะนำ
- วันสอบ
- เริ่มแสดงก่อนวันสอบกี่วัน
- CTA Label
- CTA URL
- ปักหมุด
- เปิดใช้งาน
- สถานะเผยแพร่

`exam_date` and `days_before_start` are rendered only when type is `exam_schedule`. All UI and validation messages are Thai.

## Public UI

Home loads active announcements separately from the existing `/home` contract. It renders one to three pinned/important items in a compact strip near the hero without blocking the rest of Home if the announcement request fails.

Exam-track landing pages load the track announcement endpoint and render related active announcements above the exam-set library.

Announcement detail is available at:

```text
/announcements/{slug}
```

Shared public components render type treatment, schedule countdown, summary/content, CTA, and recommended exam-set links. Public pages include loading, error, empty, and not-found handling appropriate to their context.

## Testing Strategy

Implementation follows test-first red-green-refactor cycles.

Backend unit tests cover:

- Required and conditional validation.
- CTA URL validation.
- Effective schedule start.
- Bangkok calendar `days_left` values.
- Public visibility, including pinned past schedules.
- Status transitions.
- Ordered exam-set replacement.
- Cache hit/miss and invalidation calls.
- Audit action selection.

Repository integration tests cover PostgreSQL filtering, track scope, time windows, past-exam behavior, and exam-set ordering.

Frontend unit tests cover Zod validation, countdown formatting, CTA resolution, and Home selection. A lightweight test runner is added because the current frontend has no TypeScript unit-test runner.

Final verification includes:

- Targeted Go tests and `go test ./...`.
- Frontend unit tests, ESLint, and production build.
- Desktop and mobile browser flows for Admin create/edit/status/delete, Home strip, track announcements, and detail.
- Negative paths for duplicate slug, invalid CTA, incomplete schedule publish, expired detail, and draft public access.

## Acceptance-Criteria Mapping

1. Admin CRUD and status actions are covered by Admin APIs and pages.
2. Exam schedules persist track, date, and lead time with conditional validation.
3. Home reads and renders active published announcements.
4. `days_left` and Thai countdown text are derived from Bangkok calendar dates.
5. Track pages use the scoped public endpoint.
6. Visibility policy hides expired and past schedules except the approved pinned override.
7. Track fallback CTA and ordered recommended exam sets provide related links.
8. Notification bell/read-unread state is excluded.
9. Admin UI reuses the existing full-card/data-list/form layout.
10. Indexed cache invalidation and six audit actions cover every mutation.

