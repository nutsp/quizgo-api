# Task 6 Report: Reconcile Monthly Leaderboard Data

## Status

READY FOR REVIEW

Review fixes are implemented. The progress ledger remains unchanged until review.

## Commit

- Review base: `46e3001f2c509bbaa6a3eb65ef7e46987ce38d52` (`feat: reconcile monthly leaderboard data`)
- Review-fix commit: `HEAD` at report generation; the final hash is included in the task close-out.

## Review Fixes

- Normal projection and source-of-truth reconciliation now choose the same deterministic winner for an exact score/duration/time tie: the lowest `attempt_id`.
- Added a PostgreSQL regression covering an exact tie projected in reverse ID order and rebuilt by reconciliation.
- `RecordProjectionFailure` now uses one read-committed transaction and the same Bangkok season lifecycle, exam-set transition, and season projection lock ordering as reconciliation when attempt metadata identifies a season.
- A later failure report increments retry metadata without clearing `resolved_at`, so a failure cannot reopen after a successful reconciliation.
- Added a PostgreSQL concurrency regression proving late failure recording waits for reconciliation and leaves the failure resolved.
- The reconciliation CLI rejects positional arguments after flag parsing with exit code `2` and has a focused test.

## Tests

- TDD RED: `go test ./cmd/leaderboard-reconcile -run TestRunRejectsUnexpectedPositionalArguments -count=1` failed before the fix because the command returned exit code `0` for a trailing argument.
- PASS: `go test ./cmd/leaderboard-reconcile -count=1`.
- PASS: `go test ./internal/leaderboard/... -count=1`.
- PASS: `go test -race ./internal/leaderboard/... ./cmd/leaderboard-reconcile -count=1 -timeout=8m`.
- PASS: `go test ./... -count=1 -timeout=8m`.
- PASS: `go vet ./internal/leaderboard/... ./cmd/leaderboard-reconcile`.
- PASS: targeted `gofmt -d` and whitespace checks before staging.
- PASS: exported index-only staged snapshot with `go test ./... -count=1 -timeout=8m`, the focused race lane, vet, and targeted formatting checks.
- PostgreSQL integration tests were invoked but skipped because `LEADERBOARD_POSTGRES_DSN` is not configured in this workspace. The exact-tie and concurrency regressions are present in the PostgreSQL integration suite and remain to be executed against PostgreSQL.

## Scope

The review-fix changes are limited to:

- `cmd/leaderboard-reconcile/main.go`
- `cmd/leaderboard-reconcile/main_test.go`
- `internal/leaderboard/repository/postgres.go`
- `internal/leaderboard/repository/postgres_reconcile_integration_test.go`
- `.superpowers/sdd/task-6-report.md`

The prior Task 6 files, `README.md`, `cmd/server/main.go`, and `.superpowers/sdd/progress.md` were not changed by this fix. Existing unrelated dirty and untracked work remains preserved.

## Concerns

- PostgreSQL-backed verification is unavailable until a `LEADERBOARD_POSTGRES_DSN` is supplied.
- Exact ties now use deterministic attempt-ID ordering in both paths, intentionally replacing the previous projector first-write behavior so rebuilds and incremental projection cannot disagree.
- Failure records for attempts without complete track/set/submission metadata still use the existing failure upsert without a season lock because no season can be identified from the source row.

## Blocked

No implementation blocker. PostgreSQL execution is environment-limited only.
