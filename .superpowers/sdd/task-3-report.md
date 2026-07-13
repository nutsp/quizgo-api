# Task 3 Report: Monthly Season Lifecycle And Best-Score Projection

## Status

Complete.

Implemented idempotent monthly season enrollment, half-open exam-set eligibility,
concurrency-safe best-score replacement, authoritative entry rebuilding, shared-rank
SQL, projection failure recording, and projection/lifecycle orchestration. Existing
per-exam-set and temporary track-average repository methods and queries remain intact.

## Files

- `internal/leaderboard/repository/postgres.go`
- `internal/leaderboard/repository/postgres_ranking_test.go`
- `internal/leaderboard/usecase/projector.go`
- `internal/leaderboard/usecase/projector_test.go`
- `internal/leaderboard/usecase/fakes_test.go`

## Commits

- `2f39389 feat: project monthly leaderboard scores`

## Tests And Results

### TDD red evidence

- `go test ./internal/leaderboard/usecase -run 'ProjectAttempt|Projector' -v`
  - Failed as expected because monthly repository rows and `NewProjector` were undefined.
- `go test ./internal/leaderboard/repository -run MonthlyRanking -v`
  - Failed as expected because monthly ranking SQL constants were undefined.
- `go test ./internal/leaderboard/usecase -run TestProjectAttemptRetryRepairsMissingAggregate -v`
  - Failed as expected with `entry rows = 0, want 1` and `TotalPoints = 0.0, want 90`.

### Final verification

- `go test ./internal/leaderboard/... -v`
  - PASS: domain, repository, and use-case tests; transport compiled with no tests.
- `go test -race ./internal/leaderboard/...`
  - PASS: all leaderboard packages, no detected data races.
- `git diff --check` and `git diff --cached --check`
  - PASS: no whitespace errors.
- PostgreSQL 16 ephemeral-container smoke assertions
  - PASS: `[joined_at, stopped_at)` boundaries, authoritative score aggregation,
    `RANK()` output `1,2,2,4`, and stable tied-user ordering by `user_id` outside
    the rank window.
  - The first smoke harness run failed because a PL/pgSQL assertion variable named
    `points` shadowed the test table column. The corrected, column-qualified harness passed.

## Self-Review

- Confirmed the prerequisite was exact `HEAD 5a84b8c` before implementation.
- Confirmed only the five owned leaderboard code/test files were staged and committed.
- Confirmed unrelated dirty worktree files were neither formatted, staged, nor reverted.
- Confirmed legacy per-exam-set and temporary track-average methods/queries were preserved;
  the monthly behavior was added before the existing implementation.
- Confirmed season and enrollment writes use `INSERT ... ON CONFLICT`.
- Confirmed best-score replacement uses a transaction and row-level `FOR UPDATE` lock.
- Confirmed entry rebuild locks score rows in exam-set order before aggregating and upserting.
- Confirmed every eligible retry rebuilds the entry, repairing a prior partial projection
  where the score committed but aggregate rebuilding failed.
- Confirmed ranking uses exactly total points, completed sets, total duration, and achieved
  time inside `RANK()`; `user_id` appears only in the outer stable order.
- Confirmed point normalization clamps to `0..100` and rounds to one decimal.

## Concerns

- The schema permits one enrollment row per `(season_id, exam_set_id)`. A stop followed by
  republishing in the same month cannot represent multiple disjoint eligibility intervals;
  the idempotent join reopens the existing interval. Real-time unpublished submissions are
  rejected while `stopped_at` is present, but a future reconciliation must account for this
  schema limitation if same-month republishing is a supported workflow.
- The repository methods were smoke-checked in PostgreSQL 16, but this task does not add a
  persistent database-backed Go integration test; the committed repository test focuses on
  ranking calculation and SQL policy while projector behavior is fake-backed.

## Same-Month Republish Fix

### Status

Implemented the approved interval-row design. A season/exam-set enrollment now has a UUID
primary key, preserves every closed interval, and permits at most one open interval. Repeated
publish and stop calls are idempotent, republishing inserts a new open interval immediately,
and eligibility accepts a submission only when one interval contains it using
`[joined_at, stopped_at)`. Existing best scores are not removed by stop or republish events.

The down migration is unchanged because it already drops the interval table as a whole.

### TDD And Reproduction Evidence

- Pre-fix PostgreSQL 16 reproduction:
  - Applied migration `000023` to an isolated database, inserted and stopped one interval,
    then attempted a second interval for the same season/set.
  - Failed as expected with `duplicate key value violates unique constraint
    "leaderboard_season_exam_sets_pkey"` and exit code `3`.
- `go test ./internal/leaderboard/repository -run TestSeasonExamSetModelDeclaresIntervalKeys -count=1 -v`
  - Failed as expected before the model change with `primary key = <nil>, want ID`.
- Identifier-length self-review changed the three-column uniqueness contract to the explicit,
  PostgreSQL-safe `leaderboard_season_exam_sets_interval_key` name.
  - The updated model test failed as expected with `missing
    leaderboard_season_exam_sets_interval_key index` before the migration/model alignment.
- `go test ./internal/leaderboard/repository -run 'TestSeasonExamSetModelDeclaresIntervalKeys|TestLeaderboardModelsDeclareMigrationForeignKeys' -count=1 -v`
  - PASS after the model change: UUID primary key, three-column unique index, partial unique
    open-interval index, and foreign keys all matched the migration contract.
- `go test ./internal/leaderboard/usecase -run 'TestProjectAttempt|TestProjectorManagesSeasonEnrollmentLifecycle' -count=1 -v`
  - PASS: initial interval acceptance, repeated publish no-op, repeated stop no-op, closed-gap
    rejection, exact republish-boundary acceptance, retained earlier score, and one best-score
    row after republish.

### Final Verification

- `go test ./internal/leaderboard/... -count=1 -v`
  - PASS: domain, repository, and use-case tests; transport compiled with no tests.
- `go test -race ./internal/leaderboard/... -count=1`
  - PASS: all leaderboard packages, no detected data races.
- PostgreSQL 16 isolated migration and eligibility smoke:
  - PASS: up migration; exact interval constraint and partial-index names; UUID interval rows;
    repeated open join inserted zero rows; first stop updated one row and repeated stop updated
    zero rows; first interval remained eligible; stop boundary and closed gap were rejected;
    republish created exactly a second interval; repeated republish inserted zero rows;
    republish boundary was immediately eligible; the earlier score remained; direct
    second-open insertion raised `unique_violation`; down migration removed the interval table.

### Remaining Concern

- This changes migration `000023` in place as requested. Any database that has already marked
  the old `000023` as applied needs to be recreated or upgraded with an equivalent additive
  schema migration before running this code.

## Concurrency And Rollover Fix

### Status

Implemented the Important findings and related validation Minors. Lifecycle transitions now
run in PostgreSQL transactions under one advisory lock per exam set, which is a conservative
superset of per-season/set serialization. Join uses the event timestamp to close only an older
open interval, explicitly recognizes an exact historical retry, leaves a same/newer open
interval unchanged, and inserts without generic conflict suppression. Stop closes only open
intervals whose `joined_at <= stopped_at`.

The first ensure of a Bangkok season transactionally enrolls every active published set in the
track at `starts_at`. Projection retries ensure a missing submitted-at season before checking
eligibility again. Publish-triggered season creation excludes the newly published set from the
rollover insert so its lifecycle join begins exactly at the publish event time.

Projection rejects NaN/infinite points and negative durations before repository access. Migration
and GORM model checks now enforce points in `0..100`, non-negative duration, and
`stopped_at IS NULL OR stopped_at >= joined_at`.

### TDD Evidence

- `go test ./internal/leaderboard/usecase -run 'TestProjectAttemptRejectsInvalidCandidateValues|TestProjectAttemptEnrollsActiveSetAtBangkokMonthRollover' -count=1 -v`
  - FAIL before implementation: all four invalid candidates returned a nil error; rollover
    returned `TotalPoints = 0.0` and `ensure calls = 0`.
- `go test ./internal/leaderboard/usecase -run TestProjectorLifecycleUsesEventTimeForStaleRetries -count=1 -v`
  - FAIL before implementation: `intervals after republish = 1, want 2`.
- `go test ./internal/leaderboard/usecase -run TestProjectorJoinsNewSetAtPublishTimeWhenCreatingSeason -count=1 -v`
  - FAIL before the publish-specific ensure fix: `intervals = 2, want only the publish-time interval`.
- `go test ./internal/leaderboard/repository -run 'TestSeasonExamSetModelDeclaresIntervalKeys|TestScoreModelDeclaresValueChecks|TestLifecycleSQL|TestEnsureSeasonSQL' -count=1 -v`
  - FAIL before implementation because the lifecycle lock/guard/enrollment SQL constants were
    undefined; the model declarations also lacked the three required checks.

### Focused Verification

- `go test ./internal/leaderboard/usecase -run 'TestProjectAttemptRejectsInvalidCandidateValues|TestProjectAttemptEnrollsActiveSetAtBangkokMonthRollover|TestProjectorJoinsNewSetAtPublishTimeWhenCreatingSeason|TestProjectorLifecycleUsesEventTimeForStaleRetries|TestProjectorManagesSeasonEnrollmentLifecycle' -count=1 -v -timeout=2m`
  - PASS in `0.198s`.
- `go test ./internal/leaderboard/repository -run 'TestSeasonExamSetModelDeclaresIntervalKeys|TestScoreModelDeclaresValueChecks|TestLifecycleSQLSerializesTransitionsAndGuardsEventTime|TestEnsureSeasonSQLEnrollsActivePublishedSetsAtSeasonStart|TestMonthlyRanking' -count=1 -v -timeout=2m`
  - PASS in `0.209s`, including the unchanged monthly ranking test.

### Full Verification

- `go test ./internal/leaderboard/... -v -timeout=2m`
  - PASS: domain, repository, and use-case packages; transport compiled with no tests.
- `go test -race ./internal/leaderboard/... -timeout=3m`
  - PASS: all leaderboard packages, no detected Go data races.
- `psql postgresql://appuser:appsecret@localhost:5432/virtual_exam -v ON_ERROR_STOP=1 -f migrations/000023_monthly_leaderboards.up.sql`
  - PASS against PostgreSQL 16: six tables and two indexes created.
- `LEADERBOARD_POSTGRES_DSN='postgresql://appuser:appsecret@localhost:5432/virtual_exam?sslmode=disable' go test ./internal/leaderboard/repository -run TestPostgresConcurrencyAndRolloverSmoke -count=1 -v`
  - PASS in `0.10s`: eight concurrent ensures returned one season; all prior active published
    sets were enrolled once at Bangkok month start; publish-created season setup excluded the
    publishing set until its event time; a delayed older stop did not close the newer interval;
    24 mixed concurrent joins/stops left one newest open interval; exact retries added no row;
    and the overlap query returned zero pairs.
- `psql postgresql://appuser:appsecret@localhost:5432/virtual_exam -v ON_ERROR_STOP=1 -f migrations/000023_monthly_leaderboards.down.sql`
  - PASS: all six smoke tables dropped. The temporary environment-gated Go smoke test was then
    removed and is not part of the committed test surface.

### Concerns

- Migration `000023` is still changed in place. Databases that already applied its prior form
  need recreation or an equivalent additive migration before this code is deployed.
- No persistent PostgreSQL test harness was added, per scope. Focused repository policy tests,
  PostgreSQL 16 smoke evidence, the full Go suite, and race verification cover this fix.

### Takeover Verification (2026-07-14)

This task was taken over from an interrupted worker. The intended implementation was already
committed as `43de6bd fix: serialize leaderboard lifecycle rollover`; no further production-code
change was required after independent inspection and verification.

- `git diff --check`
  - PASS: no whitespace errors.
- `go test -timeout 60s ./internal/leaderboard/usecase -run 'TestProjectAttemptRejectsInvalidCandidateValues|TestProjectAttemptEnrollsActiveSetAtBangkokMonthRollover|TestProjectorJoinsNewSetAtPublishTimeWhenCreatingSeason|TestProjectorLifecycleUsesEventTimeForStaleRetries|TestProjectorManagesSeasonEnrollmentLifecycle' -count=1 -v`
  - PASS in `0.465s`.
- `go test -timeout 60s ./internal/leaderboard/repository -run 'TestSeasonExamSetModelDeclaresIntervalKeys|TestScoreModelDeclaresValueChecks|TestLifecycleSQLSerializesTransitionsAndGuardsEventTime|TestEnsureSeasonSQLEnrollsActivePublishedSetsAtSeasonStart|TestMonthlyRanking' -count=1 -v`
  - PASS in `0.451s`.
- `go test -timeout 120s ./internal/leaderboard/... -count=1 -v`
  - PASS: domain, repository, use-case, and transport packages in `0.897s` total; no timeout or deadlock occurred.
- `go test -race -timeout 120s ./internal/leaderboard/... -count=1`
  - PASS: all leaderboard packages in `2.1s`; no Go data races reported.
- `PGPASSWORD=appsecret perl -e 'alarm 40; exec @ARGV' zsh -c '<create disposable database; apply migration 000023; exercise stale stop and one-open-interval assertions; drop database>'`
  - PASS in `0.6s`: migration applied, the stale-stop guard updated `0` rows, exactly one open interval remained at `2026-07-15T00:00:00Z`, and a second open interval raised `unique_violation`.
