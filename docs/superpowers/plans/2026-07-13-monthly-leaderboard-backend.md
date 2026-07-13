# Monthly Leaderboard Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the track-average leaderboard with an automatically managed monthly competition per exam track, project best attempt scores safely, expose hub APIs, and award permanent monthly medals.

**Architecture:** Keep `internal/leaderboard` as the owner of seasons, exam-set eligibility, score projection, aggregate entries, ranking, finalization, awards, and reconciliation. `examattempt` and `examset` depend only on small leaderboard-owned interfaces; they never write leaderboard tables directly. PostgreSQL stores read-optimized entries, while idempotent reconciliation can rebuild projections from submitted attempts.

**Tech Stack:** Go 1.24, Echo v4, GORM, PostgreSQL, UUID, existing JWT middleware and response helpers.

## Global Constraints

- Calendar boundaries use `Asia/Bangkok`.
- Every active published exam set joins the current season immediately and automatically.
- Free and member-only exam sets contribute to the same leaderboard.
- Existing points remain when an exam set is unpublished; new submissions stop counting at `stopped_at`.
- Each exam set contributes at most 100 points from the user's best eligible attempt in the month.
- Ranking order is total points descending, completed sets descending, total duration ascending, then score-achieved time ascending.
- Exact ranking ties share a rank; user ID stabilizes row order only.
- Attempt submission must remain successful if leaderboard projection fails.
- Season creation, projection, finalization, awards, and reconciliation must be idempotent.
- Keep the existing per-exam-set leaderboard API operational.

---

### Task 1: Persist Monthly Competition State

**Files:**
- Create: `migrations/000023_monthly_leaderboards.up.sql`
- Create: `migrations/000023_monthly_leaderboards.down.sql`
- Create: `internal/leaderboard/repository/models.go`
- Modify: `cmd/server/main.go:90-111`

**Interfaces:**
- Produces: GORM models `SeasonModel`, `SeasonExamSetModel`, `ScoreModel`, `EntryModel`, `AwardModel`, and `ProjectionFailureModel`.
- Produces: database uniqueness and ordering constraints consumed by all later tasks.

- [ ] **Step 1: Write the migration contract**

Create PostgreSQL tables with these required columns and constraints:

```sql
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
    season_id uuid NOT NULL REFERENCES leaderboard_seasons(id) ON DELETE CASCADE,
    exam_set_id uuid NOT NULL REFERENCES exam_sets(id),
    joined_at timestamptz NOT NULL,
    stopped_at timestamptz,
    PRIMARY KEY (season_id, exam_set_id)
);

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
```

Add `leaderboard_awards` with unique `(season_id, user_id, rank)` and `leaderboard_projection_failures` with unique `attempt_id`, retry count, last error, and resolved timestamp. The down migration drops these six tables in reverse dependency order.

- [ ] **Step 2: Add focused GORM models**

Define table names explicitly so AutoMigrate and SQL migrations agree:

```go
func (SeasonModel) TableName() string { return "leaderboard_seasons" }
func (SeasonExamSetModel) TableName() string { return "leaderboard_season_exam_sets" }
func (ScoreModel) TableName() string { return "leaderboard_scores" }
func (EntryModel) TableName() string { return "leaderboard_entries" }
func (AwardModel) TableName() string { return "leaderboard_awards" }
func (ProjectionFailureModel) TableName() string { return "leaderboard_projection_failures" }
```

- [ ] **Step 3: Register models with development AutoMigrate**

Add the six leaderboard models to `database.MustMigrate` in `cmd/server/main.go` without removing the SQL migration path used in deployed environments.

- [ ] **Step 4: Verify migration syntax and package compilation**

Run: `go test ./internal/leaderboard/... ./cmd/server`

Expected: packages compile; leaderboard packages may report `[no test files]` at this stage.

- [ ] **Step 5: Commit**

```bash
git add migrations/000023_monthly_leaderboards.* internal/leaderboard/repository/models.go cmd/server/main.go
git commit -m "feat: add monthly leaderboard schema"
```

### Task 2: Define Season, Ranking, And API Contracts

**Files:**
- Replace: `internal/leaderboard/domain/leaderboard.go`
- Create: `internal/leaderboard/domain/season_test.go`
- Create: `internal/leaderboard/domain/ranking_test.go`

**Interfaces:**
- Produces: `SeasonWindow`, `ProjectionInput`, `ProjectionUpdate`, `HubResponse`, `LeaderboardEntry`, `CurrentUserSummary`, `Award`, and `ListFilter`.
- Produces: `BangkokSeasonWindow(time.Time) (SeasonWindow, error)` and `AttemptWins(candidate, current ScoreCandidate) bool`.

- [ ] **Step 1: Write failing Bangkok boundary tests**

```go
func TestBangkokSeasonWindowUsesLocalMonth(t *testing.T) {
    input := time.Date(2026, 6, 30, 17, 30, 0, 0, time.UTC)
    got, err := BangkokSeasonWindow(input)
    require.NoError(t, err)
    assert.Equal(t, 2026, got.Year)
    assert.Equal(t, 7, got.Month)
    assert.Equal(t, time.Date(2026, 6, 30, 17, 0, 0, 0, time.UTC), got.StartsAt)
    assert.Equal(t, time.Date(2026, 7, 31, 17, 0, 0, 0, time.UTC), got.EndsAt)
}
```

Use the standard library for assertions if the repository does not already expose `testify`; do not add a new assertion dependency.

- [ ] **Step 2: Run the boundary test and confirm failure**

Run: `go test ./internal/leaderboard/domain -run TestBangkokSeasonWindowUsesLocalMonth -v`

Expected: FAIL because `BangkokSeasonWindow` is undefined.

- [ ] **Step 3: Implement deterministic domain helpers**

```go
type SeasonWindow struct {
    Year     int
    Month    int
    StartsAt time.Time
    EndsAt   time.Time
}

type ScoreCandidate struct {
    Points          float64
    DurationSeconds int
    AchievedAt      time.Time
}

func AttemptWins(candidate, current ScoreCandidate) bool {
    if candidate.Points != current.Points { return candidate.Points > current.Points }
    if candidate.DurationSeconds != current.DurationSeconds { return candidate.DurationSeconds < current.DurationSeconds }
    return candidate.AchievedAt.Before(current.AchievedAt)
}
```

Implement Bangkok month conversion with `time.LoadLocation("Asia/Bangkok")`, returning UTC boundaries.

- [ ] **Step 4: Define JSON contracts used by backend and web**

`HubResponse` must contain `season`, `exam_track`, `current_user`, `top_three`, `leaderboard`, `next_opportunities`, and `pagination`. `ProjectionUpdate` must contain:

```go
type ProjectionUpdate struct {
    SeasonID          string  `json:"season_id"`
    TrackCode         string  `json:"track_code"`
    Year              int     `json:"year"`
    Month             int     `json:"month"`
    PointsAdded       float64 `json:"points_added"`
    BestScoreBefore   float64 `json:"best_score_before"`
    BestScoreAfter    float64 `json:"best_score_after"`
    PreviousRank      *int    `json:"previous_rank,omitempty"`
    CurrentRank       int     `json:"current_rank"`
    TotalPoints       float64 `json:"total_points"`
    ImprovedBestScore bool    `json:"improved_best_score"`
}
```

- [ ] **Step 5: Run domain tests**

Run: `go test ./internal/leaderboard/domain -v`

Expected: PASS for Bangkok boundary, attempt comparison, and shared-rank fixture tests.

- [ ] **Step 6: Commit**

```bash
git add internal/leaderboard/domain
git commit -m "feat: define monthly leaderboard domain"
```

### Task 3: Implement Season Lifecycle And Score Projection

**Files:**
- Replace: `internal/leaderboard/repository/postgres.go`
- Create: `internal/leaderboard/usecase/projector.go`
- Create: `internal/leaderboard/usecase/projector_test.go`
- Create: `internal/leaderboard/usecase/fakes_test.go`

**Interfaces:**
- Produces repository methods `EnsureSeason`, `JoinExamSet`, `StopExamSet`, `GetEligibleSeason`, `UpsertBestScore`, `RebuildEntry`, `GetUserRank`, and `RecordProjectionFailure`.
- Produces `ProjectAttempt(ctx context.Context, input domain.ProjectionInput) (*domain.ProjectionUpdate, error)`.
- Produces lifecycle interface methods `OnExamSetPublished` and `OnExamSetStopped`.

- [ ] **Step 1: Write projector tests against an in-memory fake repository**

Build `projectionFixture` in `fakes_test.go` with `joinedAt`, optional `stoppedAt`, in-memory best scores, entries, and call counters. Use this table exactly:

```go
tests := []struct {
    name             string
    existing         *domain.ScoreCandidate
    submittedAt      time.Time
    candidate        domain.ScoreCandidate
    wantPoints       float64
    wantPointsAdded  float64
    wantImproved     bool
    wantScoreWrites  int
}{
    {"keeps higher existing score", score(82, 900, at(10)), at(12), candidate(75, 700, at(12)), 82, 0, false, 0},
    {"replaces tie with shorter duration", score(82, 900, at(10)), at(12), candidate(82, 800, at(12)), 82, 0, true, 1},
    {"rejects before join", nil, at(8), candidate(90, 800, at(8)), 0, 0, false, 0},
    {"rejects at stopped time", nil, at(20), candidate(90, 800, at(20)), 0, 0, false, 0},
}
```

Set fixture eligibility to `[at(9), at(20))`. Add a separate retry test that projects the same attempt ID twice and asserts one score row, one aggregate entry, and identical returned totals.

- [ ] **Step 2: Run projector tests and confirm failure**

Run: `go test ./internal/leaderboard/usecase -run ProjectAttempt -v`

Expected: FAIL because projector and repository contracts are undefined.

- [ ] **Step 3: Implement transactional repository operations**

Use `INSERT ... ON CONFLICT` for season and exam-set enrollment. Use a transaction plus `SELECT ... FOR UPDATE` for best-score replacement and entry rebuilding. Rebuild an entry from authoritative `leaderboard_scores`:

```sql
SELECT COALESCE(SUM(points), 0), COUNT(*), COALESCE(SUM(duration_seconds), 0), MAX(achieved_at)
FROM leaderboard_scores
WHERE season_id = ? AND user_id = ?;
```

Compute previous and current user ranks with `RANK() OVER` using the first four ranking fields. Sort identical displayed ranks by user ID.

- [ ] **Step 4: Implement projection orchestration**

`ProjectAttempt` must normalize points to one decimal place, check eligibility at `SubmittedAt`, update only a winning attempt, rebuild the entry, and return rank movement. A no-improvement projection returns `ImprovedBestScore: false` and zero points added.

- [ ] **Step 5: Run use-case and repository tests**

Run: `go test ./internal/leaderboard/... -v`

Expected: PASS. PostgreSQL repository package compiles and fake-backed projector tests prove the rules.

- [ ] **Step 6: Commit**

```bash
git add internal/leaderboard/repository internal/leaderboard/usecase/projector.go internal/leaderboard/usecase/*_test.go
git commit -m "feat: project monthly leaderboard scores"
```

### Task 4: Connect Publication And Attempt Submission Safely

**Files:**
- Modify: `internal/examset/usecase/admin.go:17-42`
- Modify: `internal/examset/usecase/publish.go:144-216`
- Create: `internal/examset/usecase/leaderboard_lifecycle_test.go`
- Modify: `internal/examattempt/domain/examattempt.go:153-164`
- Modify: `internal/examattempt/usecase/examattempt.go:28-77,370-480,793-814`
- Create: `internal/examattempt/usecase/leaderboard_projection_test.go`
- Modify: `cmd/server/main.go:132-161,181-196`

**Interfaces:**
- Consumes: `ProjectAttempt`, `OnExamSetPublished`, and `OnExamSetStopped` from Task 3.
- Produces: optional `competition_update` in `domain.SubmitResponse`.

- [ ] **Step 1: Write failing lifecycle and submission tests**

Use narrow interfaces owned by the consuming packages:

```go
type LeaderboardProjector interface {
    ProjectAttempt(context.Context, leaderboarddomain.ProjectionInput) (*leaderboarddomain.ProjectionUpdate, error)
    RecordProjectionFailure(context.Context, uuid.UUID, error) error
}

type LeaderboardLifecycle interface {
    OnExamSetPublished(context.Context, uuid.UUID, uuid.UUID, time.Time) error
    OnExamSetStopped(context.Context, uuid.UUID, time.Time) error
}
```

Tests must prove publish calls join after the status update, unpublish/archive calls stop, a successful submission remains successful when projection returns an error, a projection failure is recorded with the attempt ID, and an attempt automatically marked `timeout` is projected exactly once.

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `go test ./internal/examset/usecase ./internal/examattempt/usecase -run Leaderboard -v`

Expected: FAIL because constructors do not accept the new interfaces.

- [ ] **Step 3: Add lifecycle hooks**

Inject `LeaderboardLifecycle` into `AdminUseCase`. Call it with `time.Now().UTC()` after successful publish/unpublish/archive persistence. Return a lifecycle error from publication actions so the admin sees that enrollment did not complete; idempotency makes retry safe.

- [ ] **Step 4: Add non-blocking attempt projection**

After `UpdateAttemptSubmitted` succeeds, call the projector. Also project the stale-attempt path after `MarkAttemptTimeout` persists its submission timestamp; a timed-out attempt with no calculated score contributes zero points but still counts as one completed exam set under the approved eligible-status rule. Attach a successful update to:

```go
type SubmitResponse struct {
    // existing fields
    CompetitionUpdate *leaderboarddomain.ProjectionUpdate `json:"competition_update,omitempty"`
}
```

If projection fails, call `RecordProjectionFailure`; log the error and continue returning the valid exam submission response.

- [ ] **Step 5: Wire dependencies in server composition**

Construct leaderboard repository/use case before attempt and admin exam-set use cases, inject the interfaces, and keep route registration after Echo creation.

- [ ] **Step 6: Run focused and package-wide tests**

Run: `go test ./internal/examset/usecase ./internal/examattempt/usecase ./internal/leaderboard/... -v`

Expected: PASS, including projection failure not changing submission success.

- [ ] **Step 7: Commit**

```bash
git add internal/examset/usecase internal/examattempt cmd/server/main.go
git commit -m "feat: connect exams to monthly leaderboard"
```

### Task 5: Expose Competition Hub And Award APIs

**Files:**
- Create: `internal/leaderboard/usecase/hub.go`
- Create: `internal/leaderboard/usecase/hub_test.go`
- Create: `internal/leaderboard/usecase/finalizer.go`
- Create: `internal/leaderboard/usecase/finalizer_test.go`
- Replace: `internal/leaderboard/transport/http/handler.go`
- Create: `internal/leaderboard/transport/http/handler_test.go`
- Create: `internal/leaderboard/transport/http/rate_limiter.go`
- Create: `internal/leaderboard/transport/http/rate_limiter_test.go`
- Modify: `internal/leaderboard/usecase/leaderboard.go`
- Modify: `cmd/server/main.go:181-183`

**Interfaces:**
- Produces: `GetOverview`, `GetHub`, `ListAwards`, and `FinalizeDueSeasons`.
- Produces authenticated routes `/leaderboards/overview`, `/leaderboards/exam-tracks/:trackCode`, and `/me/leaderboard-awards`.
- Preserves `/exam-sets/:examSetCode/leaderboard`.

- [ ] **Step 1: Write use-case tests for hub modes and awards**

Cover default recent track selection, explicit year/month, `scope=top`, `scope=around_me`, unranked user, finalized season, next opportunities, disabled-user omission from active rows, and shared-rank medal assignment.

- [ ] **Step 2: Run hub tests and confirm failure**

Run: `go test ./internal/leaderboard/usecase -run 'Hub|Finalize|Award' -v`

Expected: FAIL because hub and finalizer methods are undefined.

- [ ] **Step 3: Implement read queries and finalization transaction**

Top mode defaults to 20 and caps at 100. Around-me mode returns five rows above and below the current user. Finalization locks the season, recomputes final ranks, inserts awards for displayed ranks 1-3 with `ON CONFLICT DO NOTHING`, then marks the season finalized.

- [ ] **Step 4: Write HTTP contract tests**

Verify authentication, valid query parsing, invalid year/month returning validation errors, `scope` validation, pagination, and stable JSON keys. Register:

```go
g.GET("/leaderboards/overview", h.GetOverview, authMiddleware)
g.GET("/leaderboards/exam-tracks/:trackCode", h.GetHub, authMiddleware)
g.GET("/me/leaderboard-awards", h.ListMyAwards, authMiddleware)
```

- [ ] **Step 5: Implement handlers and retain per-set endpoint**

Remove the old track-average implementation from public routing only after the new hub route is registered. Keep its response types only if the web migration still needs a temporary compatibility window.

- [ ] **Step 6: Add a Redis-backed authenticated read limit**

Implement a fixed-window limiter keyed by user ID and minute using the existing runtime Redis client. Allow 120 leaderboard reads per authenticated user per minute; fail open and log when Redis is unavailable so rankings never block exam submission or result access. Handler tests must prove the 121st request returns HTTP 429 and a new minute resets the allowance.

- [ ] **Step 7: Run leaderboard tests**

Run: `go test ./internal/leaderboard/... -v`

Expected: PASS for domain, projection, hub, finalization, and HTTP behavior.

- [ ] **Step 8: Commit**

```bash
git add internal/leaderboard
git commit -m "feat: expose monthly competition APIs"
```

### Task 6: Add Reconciliation And Current-Month Backfill

**Files:**
- Create: `internal/leaderboard/usecase/reconcile.go`
- Create: `internal/leaderboard/usecase/reconcile_test.go`
- Create: `cmd/leaderboard-reconcile/main.go`
- Modify: `internal/leaderboard/repository/postgres.go`
- Modify: `README.md`

**Interfaces:**
- Produces: `ReconcileSeason(ctx context.Context, trackID uuid.UUID, at time.Time) (ReconcileSummary, error)`.
- Produces CLI flags `-track-code`, `-year`, `-month`, and `-all-active-tracks`.

- [ ] **Step 1: Write reconciliation tests**

Use fixtures with repeated attempts, attempts before a mid-month join, attempts after a stop, timeout attempts, and duplicate retries. Assert the rebuilt scores and entries exactly match normal projection output.

- [ ] **Step 2: Run reconciliation tests and confirm failure**

Run: `go test ./internal/leaderboard/usecase -run Reconcile -v`

Expected: FAIL because `ReconcileSeason` is undefined.

- [ ] **Step 3: Implement source-of-truth rebuild**

Within one season-scoped transaction, select eligible `submitted` and `timeout` attempts, choose each user's best attempt per set with the same score/duration/time ordering, replace projections for the season, rebuild entries, resolve matching projection failures, and return counts.

- [ ] **Step 4: Add the operational command**

The command must require either one track code or `-all-active-tracks`, reject future months, print season ID plus score/entry counts, and exit non-zero on any track failure.

- [ ] **Step 5: Document deployment order**

Add exact commands:

```bash
go run ./cmd/leaderboard-reconcile -all-active-tracks -year 2026 -month 7
go test ./internal/leaderboard/... ./internal/examattempt/usecase ./internal/examset/usecase
```

- [ ] **Step 6: Run reconciliation and command tests**

Run: `go test ./internal/leaderboard/... ./cmd/leaderboard-reconcile -v`

Expected: PASS and command package compiles.

- [ ] **Step 7: Commit**

```bash
git add internal/leaderboard cmd/leaderboard-reconcile README.md
git commit -m "feat: reconcile monthly leaderboard data"
```

### Task 7: Backend Verification And Contract Freeze

**Files:**
- Modify only files required to fix verification failures from Tasks 1-6.

**Interfaces:**
- Produces the frozen API contract consumed by the web implementation plan.

- [ ] **Step 1: Format changed Go files**

Run: `gofmt -w internal/leaderboard internal/examattempt internal/examset cmd/server cmd/leaderboard-reconcile`

Expected: no output.

- [ ] **Step 2: Run focused tests with race detection**

Run: `go test -race ./internal/leaderboard/... ./internal/examattempt/usecase ./internal/examset/usecase`

Expected: PASS with no data races.

- [ ] **Step 3: Run the full API test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 4: Inspect migration reversibility**

Apply migrations through `000023`, run the reconciliation command against a disposable database, inspect one hub response, then roll back `000023` and confirm all six leaderboard tables are removed without affecting attempts.

- [ ] **Step 5: Commit verification fixes**

```bash
git add internal cmd migrations README.md
git commit -m "test: verify monthly leaderboard backend"
```
