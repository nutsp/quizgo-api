# Announcement Feature with Exam Schedule Alerts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Admin-managed QuizGo announcements, exam-date countdown alerts, related track/exam-set CTAs, cached public APIs, and Thai Admin/Public UI.

**Architecture:** Add an `announcement` vertical slice to the Go API with PostgreSQL as source of truth, use-case-owned validation/visibility, indexed Redis cache invalidation, and thin Echo handlers. Add feature-owned TypeScript API contracts and shared presentation components to the Next.js app while reusing the existing Admin full-card/data-list/form system.

**Tech Stack:** Go 1.24, Echo, GORM, PostgreSQL, Redis; Next.js App Router, TypeScript, React Hook Form, Zod, TailwindCSS, shadcn/ui, lucide-react.

## Global Constraints

- UI text is Thai.
- Public APIs expose only published, active, currently visible announcements.
- `exam_schedule` requires `exam_track_id` and `exam_date`.
- A null `starts_at` uses `exam_date - days_before_start` as the effective start in `Asia/Bangkok`.
- The Admin form does not expose `priority`, `starts_at`, or `ends_at`.
- Past exam schedules are hidden unless pinned; pinning does not bypass other visibility rules.
- No notification bell or read/unread state in MVP.
- Use Redis set indexes for invalidation; never use Redis `KEYS`.
- Preserve existing Home and exam-library contracts and behavior.
- Work feature-first: add only targeted backend tests for visibility/validation risk; do not add a frontend test framework.

---

### Task 1: Persist announcements and define domain rules

**Files:**
- Create: `migrations/000027_announcements.up.sql`
- Create: `migrations/000027_announcements.down.sql`
- Create: `internal/announcement/domain/announcement.go`
- Create: `internal/announcement/domain/visibility_test.go`
- Modify: `internal/apperrors/errors.go`

**Interfaces:**
- Produces: `domain.Announcement`, `domain.Type`, `domain.PublishStatus`, `domain.VisibilityAt(now time.Time) bool`, `domain.DaysLeft(now time.Time) *int`.
- Produces: `apperrors.ErrAnnouncementNotFound`, `ErrAnnouncementSlugTaken`, and `ErrAnnouncementInvalidStatus`.

- [ ] **Step 1: Add targeted failing visibility tests**

```go
func TestExamScheduleVisibilityUsesBangkokCalendar(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Bangkok")
	examDate := time.Date(2026, 8, 15, 0, 0, 0, 0, location)
	a := domain.Announcement{
		Type: domain.TypeExamSchedule, PublishStatus: domain.StatusPublished,
		IsActive: true, ExamDate: &examDate, DaysBeforeStart: 14,
	}

	if a.VisibleAt(time.Date(2026, 7, 31, 12, 0, 0, 0, location)) {
		t.Fatal("announcement became visible before its derived start date")
	}
	if !a.VisibleAt(time.Date(2026, 8, 1, 0, 0, 0, 0, location)) {
		t.Fatal("announcement was hidden on its derived start date")
	}
	daysLeft := a.DaysLeft(time.Date(2026, 8, 1, 12, 0, 0, 0, location))
	if daysLeft == nil || *daysLeft != 14 {
		t.Fatalf("days left = %v, want 14", daysLeft)
	}
}
```

- [ ] **Step 2: Run the focused test and confirm the missing package failure**

Run: `cd api && go test ./internal/announcement/domain -run TestExamScheduleVisibilityUsesBangkokCalendar -v`

Expected: FAIL because `internal/announcement/domain` does not exist yet.

- [ ] **Step 3: Add the migration and domain model**

The migration must create the two tables and exact constraints from the approved design. The domain model must expose these constants:

```go
const (
	TypeGeneral Type = "general"
	TypeExamSchedule Type = "exam_schedule"
	TypeExamUpdate Type = "exam_update"
	TypePromotion Type = "promotion"
	TypeMaintenance Type = "maintenance"
	TypeSystem Type = "system"

	StatusDraft PublishStatus = "draft"
	StatusPublished PublishStatus = "published"
	StatusArchived PublishStatus = "archived"
)
```

`VisibleAt` must enforce status, active state, explicit time windows, derived schedule start, and the pinned past-date exception. `DaysLeft` must compare Bangkok calendar dates rather than durations.

- [ ] **Step 4: Add announcement errors**

```go
ErrAnnouncementNotFound = New("ANNOUNCEMENT_NOT_FOUND", "ไม่พบประกาศ", http.StatusNotFound)
ErrAnnouncementSlugTaken = New("ANNOUNCEMENT_SLUG_TAKEN", "Slug นี้ถูกใช้งานแล้ว", http.StatusConflict)
ErrAnnouncementInvalidStatus = New("ANNOUNCEMENT_INVALID_STATUS", "สถานะประกาศไม่ถูกต้อง", http.StatusBadRequest)
```

- [ ] **Step 5: Run the focused domain test**

Run: `cd api && go test ./internal/announcement/domain -v`

Expected: PASS.

- [ ] **Step 6: Commit the schema/domain slice**

```bash
cd api
git add migrations/000027_announcements.* internal/announcement/domain internal/apperrors/errors.go
git commit -m "feat: add announcement schema and schedule rules"
```

### Task 2: Build repository, validation, public reads, and cache policy

**Files:**
- Create: `internal/announcement/repository/postgres.go`
- Create: `internal/announcement/usecase/announcement.go`
- Create: `internal/announcement/usecase/validation.go`
- Create: `internal/announcement/usecase/validation_test.go`
- Modify: `internal/cache/keys.go`
- Modify: `internal/cache/invalidator.go`

**Interfaces:**
- Produces repository methods `ListAdmin`, `FindByID`, `FindBySlug`, `Create`, `Update`, `ReplaceExamSets`, `UpdateStatus`, `Delete`, `ListActive`, and `ListActiveByTrackCode`.
- Produces use-case methods matching all six Admin endpoints and three Public endpoints.
- Produces cache keys `AnnouncementsActive`, `AnnouncementsActiveByType`, `AnnouncementsByTrack`, and index `IndexAnnouncements`.

- [ ] **Step 1: Add focused validation tests**

Cover required title/slug/type, conditional schedule fields, nonnegative lead time, case-insensitive slug format, CTA URL rules, and start/end order. Use table-driven tests around:

```go
func ValidateInput(input MutationInput) error
```

- [ ] **Step 2: Run validation tests and confirm RED**

Run: `cd api && go test ./internal/announcement/usecase -run TestValidateInput -v`

Expected: FAIL because `ValidateInput` is not implemented.

- [ ] **Step 3: Implement repository models and transactional writes**

Use `AnnouncementModel` and `AnnouncementExamSetModel` with explicit `TableName` methods. `Create` and `Update` must use a GORM transaction that replaces recommendation rows in incoming array order. Public queries must preload only exam sets satisfying:

```sql
exam_sets.status = 'published' AND exam_sets.is_active = true
```

Admin list supports `q`, `type`, `publish_status`, `is_active`, pagination, and safe sort/order.

- [ ] **Step 4: Implement validation and use-case orchestration**

Use this request shape:

```go
type MutationInput struct {
	Title string `json:"title"`
	Slug string `json:"slug"`
	Summary string `json:"summary"`
	Content string `json:"content"`
	Type domain.Type `json:"type"`
	Priority int `json:"priority"`
	IsPinned bool `json:"is_pinned"`
	IsActive bool `json:"is_active"`
	PublishStatus domain.PublishStatus `json:"publish_status"`
	StartsAt *time.Time `json:"starts_at"`
	EndsAt *time.Time `json:"ends_at"`
	ExamTrackID *uuid.UUID `json:"exam_track_id"`
	ExamDate *time.Time `json:"exam_date"`
	DaysBeforeStart int `json:"days_before_start"`
	CTALabel string `json:"cta_label"`
	CTAURL string `json:"cta_url"`
	ExamSetIDs []uuid.UUID `json:"exam_set_ids"`
}
```

Create/update validate referenced track/exam-set IDs and slug uniqueness. Public result mapping calculates `days_left`, `schedule_text`, fallback CTA, related track, and filtered exam sets.

- [ ] **Step 5: Add indexed cache behavior**

```go
const TTLAnnouncements = 5 * time.Minute

func AnnouncementsActive() string { return "announcements:active" }
func AnnouncementsActiveByType(t string) string { return "announcements:active:type:" + t }
func AnnouncementsByTrack(slug string) string { return "announcements:track:" + slug }
func IndexAnnouncements() string { return "index:announcements" }
```

Every public list cache fill registers in `IndexAnnouncements`. Add `OnAnnouncementChanged(ctx)` to delete that index. Reapply `VisibleAt` after cache reads so stale records disappear immediately at an end/exam boundary; the five-minute TTL discovers newly active records.

- [ ] **Step 6: Run targeted use-case and cache tests**

Run: `cd api && go test ./internal/announcement/... ./internal/cache/...`

Expected: PASS.

- [ ] **Step 7: Commit repository/use-case/cache slice**

```bash
cd api
git add internal/announcement internal/cache
git commit -m "feat: implement announcement business flow"
```

### Task 3: Expose Admin/Public HTTP APIs, audit logs, and server wiring

**Files:**
- Create: `internal/announcement/transport/http/handler.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: announcement repository/use case from Task 2.
- Produces exactly the nine routes in the approved API contract.

- [ ] **Step 1: Implement route registration and thin handlers**

```go
func (h *Handler) RegisterPublicRoutes(api *echo.Group) {
	api.GET("/announcements/active", h.ListActive)
	api.GET("/announcements/:slug", h.GetPublic)
	api.GET("/exam-tracks/:trackSlug/announcements", h.ListByTrack)
}

func (h *Handler) RegisterAdminRoutes(admin *echo.Group) {
	admin.GET("/announcements", h.ListAdmin)
	admin.POST("/announcements", h.Create)
	admin.GET("/announcements/:id", h.GetAdmin)
	admin.PATCH("/announcements/:id", h.Update)
	admin.PATCH("/announcements/:id/status", h.UpdateStatus)
	admin.DELETE("/announcements/:id", h.Delete)
}
```

Handlers parse UUID/query/body, call one use-case method, and return through `response.JSON`/`response.Error`.

- [ ] **Step 2: Add mutation audit mapping**

After each successful mutation, log resource type `announcement` and one of:

```go
announcement.create
announcement.update
announcement.publish
announcement.unpublish
announcement.archive
announcement.delete
```

Resolve actor data using the same authenticated-user helper pattern used by existing Admin handlers. Include before/after snapshots for update/status and before snapshot for delete.

- [ ] **Step 3: Wire models, repositories, use cases, and routes**

Add announcement models to `database.MustMigrate`, instantiate the repository/use case with `contentCache`, `cacheInvalidator`, track/exam-set repositories, then register public routes on `api` and Admin routes on `adminRoute`.

- [ ] **Step 4: Format and compile all backend packages**

Run: `cd api && gofmt -w internal/announcement internal/cache internal/apperrors cmd/server/main.go && go test ./...`

Expected: PASS with exit code 0.

- [ ] **Step 5: Commit the HTTP/wiring slice**

```bash
cd api
git add cmd/server/main.go internal/announcement internal/cache internal/apperrors
git commit -m "feat: expose announcement APIs and audit actions"
```

### Task 4: Add shared frontend announcement contracts and public components

**Files:**
- Create: `web/src/lib/api/announcementApi.ts`
- Create: `web/src/lib/announcement/format.ts`
- Create: `web/src/components/announcements/AnnouncementStrip.tsx`
- Create: `web/src/components/announcements/AnnouncementCard.tsx`
- Create: `web/src/components/announcements/AnnouncementDetail.tsx`

**Interfaces:**
- Produces `Announcement`, `AnnouncementType`, `AnnouncementPublishStatus`, `announcementApi.listActive`, `getBySlug`, and `listByTrack`.
- Produces `resolveAnnouncementCTA` and Thai label maps shared by all public surfaces.

- [ ] **Step 1: Define API types and calls**

```ts
export type AnnouncementType =
  | "general" | "exam_schedule" | "exam_update"
  | "promotion" | "maintenance" | "system";

export type Announcement = {
  id: string;
  title: string;
  slug: string;
  summary?: string;
  content?: string;
  type: AnnouncementType;
  priority: number;
  is_pinned: boolean;
  exam_date?: string | null;
  days_left?: number | null;
  schedule_text?: string | null;
  cta_label?: string | null;
  cta_url?: string | null;
  exam_track?: { id: string; code: string; name: string } | null;
  recommended_exam_sets: Array<{ id: string; code: string; title: string }>;
};
```

Use unauthenticated `apiGet` for all public calls.

- [ ] **Step 2: Build shared Thai announcement presentation**

`AnnouncementStrip` renders one to three important items. `AnnouncementCard` renders type badge, countdown, summary, and CTA. `AnnouncementDetail` renders content and recommended exam-set links. All links use Next `Link`, provide visible focus styles, and remain usable on mobile.

- [ ] **Step 3: Run frontend lint for the new shared files**

Run: `cd web && npx eslint src/lib/api/announcementApi.ts src/lib/announcement/format.ts src/components/announcements`

Expected: PASS.

- [ ] **Step 4: Commit the shared public frontend slice**

```bash
cd web
git add src/lib/api/announcementApi.ts src/lib/announcement src/components/announcements
git commit -m "feat: add announcement frontend contracts"
```

### Task 5: Build Admin announcement form and API client

**Files:**
- Modify: `web/src/lib/api/admin/endpoints.ts`
- Create: `web/src/components/admin/forms/AnnouncementForm.tsx`
- Create: `web/src/app/(admin)/admin/announcements/new/page.tsx`
- Create: `web/src/app/(admin)/admin/announcements/[id]/edit/page.tsx`
- Modify: `web/src/lib/admin/breadcrumbs.ts`
- Modify: `web/src/components/admin/layout/AdminSidebar.tsx`

**Interfaces:**
- Produces `AdminAnnouncement`, `AnnouncementInput`, `adminAnnouncementsApi`.
- Produces form ID `admin-announcement-form` and create/edit pages.

- [ ] **Step 1: Add Admin API types and methods**

```ts
export const adminAnnouncementsApi = {
  list: (params) => apiGet(`/admin/announcements${listQuery(params)}`, true),
  get: (id: string) => apiGet(`/admin/announcements/${id}`, true),
  create: (input: AnnouncementInput) => apiPost("/admin/announcements", input, true),
  update: (id: string, input: AnnouncementInput) => apiPatch(`/admin/announcements/${id}`, input, true),
  updateStatus: (id: string, publish_status: AnnouncementPublishStatus) =>
    apiPatch(`/admin/announcements/${id}/status`, { publish_status }, true),
  delete: (id: string) => apiDelete(`/admin/announcements/${id}`, true),
};
```

- [ ] **Step 2: Build the Zod/RHF form**

The schema validates all approved fields and uses `superRefine` for schedule requirements. When type changes away from `exam_schedule`, clear `exam_track_id`, `exam_date`, and reset `days_before_start` to zero. Populate track and exam-set selects from existing Admin APIs. Keep `priority`, `starts_at`, and `ends_at` out of the UI and request mapping.

- [ ] **Step 3: Build create/edit pages using the existing full-card form layout**

Use `AdminFormPageCard`, `AdminFormMetadataRow`, Thai toasts, loading/not-found states, and redirect to `/admin/announcements` after success.

- [ ] **Step 4: Add sidebar and breadcrumb entries**

Add `ประกาศ` with the `Megaphone` icon under `จัดการเนื้อหา`, plus list/create/edit breadcrumb helpers and back link.

- [ ] **Step 5: Lint the Admin form slice**

Run: `cd web && npx eslint 'src/app/(admin)/admin/announcements' src/components/admin/forms/AnnouncementForm.tsx src/lib/api/admin/endpoints.ts src/lib/admin/breadcrumbs.ts src/components/admin/layout/AdminSidebar.tsx`

Expected: PASS.

- [ ] **Step 6: Commit the Admin form slice**

```bash
cd web
git add src/lib/api/admin/endpoints.ts src/components/admin/forms/AnnouncementForm.tsx 'src/app/(admin)/admin/announcements' src/lib/admin/breadcrumbs.ts src/components/admin/layout/AdminSidebar.tsx
git commit -m "feat: add announcement admin form"
```

### Task 6: Build Admin announcement list and actions

**Files:**
- Create: `web/src/components/admin/announcements/AnnouncementList.tsx`
- Create: `web/src/components/admin/announcements/AnnouncementListItem.tsx`
- Create: `web/src/app/(admin)/admin/announcements/page.tsx`
- Modify: `web/src/components/admin/common/admin-list-columns.ts`
- Modify: `web/src/lib/admin/data-list.ts`
- Modify: `web/src/app/globals.css`
- Modify: `web/src/lib/admin/labels.ts`

**Interfaces:**
- Produces `ANNOUNCEMENT_GRID`, `ANNOUNCEMENT_MIN_WIDTH`, desktop/mobile rows, and all requested list actions.

- [ ] **Step 1: Add the Admin list grid**

Define a static `admin-grid-announcement` class covering row number plus the requested columns. Export it from both Admin grid modules so Tailwind sees literal class names.

- [ ] **Step 2: Implement desktop/mobile list rows**

Render title, Thai type label, exam track, exam date, publish badge, pinned state, starts/ends, latest update, and compact action controls. Reuse `AdminPublishStatusBadge`, `AdminDateCell`, `AdminIconButton`, and row primitives.

- [ ] **Step 3: Implement list state and mutation actions**

Use `useAdminPaginatedList` and existing URL filter hooks. Provide search/type/status filters. Confirm archive/delete through `AdminConfirmDialog`; publish/unpublish can run directly with loading protection and Thai toasts. Reload after mutations.

- [ ] **Step 4: Add audit labels**

Map all six announcement audit actions to clear Thai labels in `src/lib/admin/labels.ts`.

- [ ] **Step 5: Lint the Admin list slice**

Run: `cd web && npx eslint 'src/app/(admin)/admin/announcements/page.tsx' src/components/admin/announcements src/components/admin/common/admin-list-columns.ts src/lib/admin/data-list.ts src/lib/admin/labels.ts`

Expected: PASS.

- [ ] **Step 6: Commit the Admin list slice**

```bash
cd web
git add 'src/app/(admin)/admin/announcements/page.tsx' src/components/admin/announcements src/components/admin/common/admin-list-columns.ts src/lib/admin/data-list.ts src/app/globals.css src/lib/admin/labels.ts
git commit -m "feat: add announcement admin management"
```

### Task 7: Integrate Home, track pages, and public detail

**Files:**
- Modify: `web/src/components/home/HomePageClient.tsx`
- Modify: `web/src/components/exams/ExamLibraryPageClient.tsx`
- Modify: `web/src/app/exams/_track/createExamTrackPage.tsx`
- Create: `web/src/app/announcements/[slug]/page.tsx`
- Create: `web/src/app/announcements/[slug]/loading.tsx`
- Create: `web/src/app/announcements/[slug]/not-found.tsx`

**Interfaces:**
- Consumes: public announcement API/components from Task 4.
- Produces Home strip, track-scoped announcements, and `/announcements/{slug}`.

- [ ] **Step 1: Load Home announcements independently**

Fetch `announcementApi.listActive()` alongside but independently from `/home`. Select at most three items with `is_pinned || priority > 0`. Render below the hero; on failure render nothing so Home remains usable.

- [ ] **Step 2: Load track-scoped announcements**

Pass `announcementTrackSlug={track.code}` from `createExamTrackPage` into `ExamLibraryPageClient`. Fetch the track endpoint and render cards after the page intro and before filters. Treat request failure as an empty optional section.

- [ ] **Step 3: Build the public detail route**

Use a server page with metadata from the announcement title/summary. Convert API 404 to `notFound()`. Render shared detail content, CTA, recommended exam sets, Thai loading state, and a Thai not-found page.

- [ ] **Step 4: Run full frontend checks**

Run: `cd web && npm run lint && npm run build`

Expected: both commands exit 0.

- [ ] **Step 5: Commit the public integration slice**

```bash
cd web
git add src/components/home/HomePageClient.tsx src/components/exams/ExamLibraryPageClient.tsx src/app/exams/_track/createExamTrackPage.tsx src/app/announcements
git commit -m "feat: show active announcements publicly"
```

### Task 8: Verify end-to-end behavior and update project memory

**Files:**
- Modify: `.cursor/memory/project-memory.md`

**Interfaces:**
- Produces durable project memory with routes, visibility rules, cache keys, audit actions, and UI locations.

- [ ] **Step 1: Run fresh backend verification**

Run: `cd api && go test ./...`

Expected: PASS with zero failures.

- [ ] **Step 2: Run fresh frontend verification**

Run: `cd web && npm run lint && npm run build`

Expected: PASS with zero ESLint/build errors.

- [ ] **Step 3: Exercise browser flows on desktop and mobile**

Verify Admin create/edit for a general and schedule announcement, publish/unpublish/archive/delete, Home maximum-three strip, track scoping, detail CTA, `สอบวันนี้`, one-day/multi-day countdowns, and absence of draft/expired content. Capture screenshots only when the local API/database can run with representative data.

- [ ] **Step 4: Confirm cache and audit behavior**

Inspect Redis with direct `GET`/`SMEMBERS` for the three announcement key families and `index:announcements`; confirm mutations clear indexed keys. Query Admin Audit Logs for all six action names. Do not use `KEYS` during verification.

- [ ] **Step 5: Update project memory**

Append an `Announcement Feature (2026-07)` section to `.cursor/memory/project-memory.md` describing database tables, API routes, Bangkok date rules, Admin/Public routes, cache invalidation, audit actions, verification evidence, and any remaining risk.

- [ ] **Step 6: Inspect final diffs and repository state**

Run:

```bash
cd api && git status --short && git diff --check HEAD~3..HEAD
cd ../web && git status --short && git diff --check HEAD~4..HEAD
```

Expected: no uncommitted feature files and no whitespace errors.

- [ ] **Step 7: Commit documentation in the owning repositories**

Commit API knowledge changes in `api` if present. Because `.cursor/memory/project-memory.md` lives outside both Git repositories, leave it updated in the shared workspace and report that fact explicitly.
