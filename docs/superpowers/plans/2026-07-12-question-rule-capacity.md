# Question Rule Capacity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show and enforce the maximum eligible question count for every exam-set question rule.

**Architecture:** The repository counts eligible and assigned questions with the same subject/tag/difficulty predicates used by assignment. A capacity endpoint previews draft rules, while rule saving runs the same authoritative validation. The React panel debounces previews, displays all three counts, and blocks invalid saves.

**Tech Stack:** Go, GORM, PostgreSQL, Echo, Next.js 16, React, TypeScript.

## Global Constraints

- Count only published, active questions.
- `total_available` includes questions already assigned to the current exam set.
- `remaining_available = total_available - assigned_count`.
- Keep draft rule values intact while capacity refreshes.
- Preserve unrelated dirty-worktree changes.

---

### Task 1: Capacity Query and API

**Files:**
- Modify: `internal/examsetquestion/repository/postgres.go`
- Modify: `internal/examsetquestion/usecase/usecase.go`
- Modify: `internal/examsetquestion/transport/http/handler.go`

**Interfaces:**
- Produces: `QuestionRuleCapacity(ctx, examSetID, selection) (total, assigned int64, err error)`.
- Produces: `POST /admin/exam-sets/:id/question-rules/capacity` preserving submitted rule order.

- [ ] Add a repository count query using subject, optional tag, optional difficulty, published status, and active state.
- [ ] Add capacity request/response DTOs with `total_available`, `assigned_count`, and `remaining_available`.
- [ ] Register and implement the capacity HTTP handler.
- [ ] Run `go test ./internal/examsetquestion/...` and expect PASS.

### Task 2: Authoritative Save Validation

**Files:**
- Modify: `internal/examsetquestion/usecase/usecase.go`
- Test: `internal/examsetquestion/usecase/capacity_test.go`

**Interfaces:**
- Consumes: repository capacity counts.
- Produces: save rejection when `rule.Count > total_available`.

- [ ] Add a failing unit test for a count above capacity and equality at capacity.
- [ ] Validate every parsed rule before replacing persisted rules.
- [ ] Return a Thai validation message containing the one-based rule index and maximum.
- [ ] Run `go test ./internal/examsetquestion/usecase -v` and expect PASS.

### Task 3: Capacity UI

**Files:**
- Modify: `web/src/lib/api/admin/exam-set-questions.ts`
- Modify: `web/src/components/admin/exam-set-questions/AutoAssignPanel.tsx`
- Modify: `web/src/app/(admin)/admin/exam-sets/[id]/questions/page.tsx`

**Interfaces:**
- Consumes: ordered draft rules.
- Produces: capacity row metadata and `max` numeric inputs.

- [ ] Add frontend capacity request/response types and API method.
- [ ] Debounce capacity loading after condition changes without changing draft counts.
- [ ] Display `มีทั้งหมด N · อยู่ในชุดแล้ว A · เพิ่มได้อีก R` under each rule.
- [ ] Set input `max`, show an inline over-capacity error, and disable save/auto while invalid or loading fails.
- [ ] Run `npm run lint && npm run build` and expect no errors.

### Task 4: End-to-End Verification

**Files:**
- No new production files.

- [ ] Run `go test ./...`.
- [ ] Run `npm run lint && npm run build`.
- [ ] Restart the local API and verify the capacity endpoint returns ordered counts.
- [ ] Open the admin page and verify capacity text, max validation, and button states visually.
