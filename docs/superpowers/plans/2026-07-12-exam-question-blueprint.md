# Exam Question Blueprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the exam set define one question difficulty and make its saved blueprint control both automatic and manual question assignment.

**Architecture:** Keep the existing rule table for backward compatibility, but normalize every rule to the current exam-set difficulty inside the use case. Remove rule-level difficulty from the HTTP contract and Admin editor, while retaining repository fields so no migration is required.

**Tech Stack:** Go, Echo, GORM, Next.js 16, React 19, TypeScript, TailwindCSS.

## Global Constraints

- The user-facing term is `โครงสร้างชุดข้อสอบ`, not `เงื่อนไขการจัดคำถาม`.
- A blueprint row contains subject, optional question group, and count.
- Difficulty always comes from the exam set.
- Automatic and manual assignment must obey the same backend validation.
- Preserve existing dirty-worktree changes.

---

### Task 1: Backend Blueprint Source Of Truth

**Files:**
- Modify: `internal/examsetquestion/usecase/usecase.go`
- Test: `internal/examsetquestion/usecase/blueprint_difficulty_test.go`

**Interfaces:**
- Consumes: `esdomain.ExamSet.Difficulty` and parsed `[]esqdomain.QuestionRule`.
- Produces: `applyExamSetDifficulty(rules []esqdomain.QuestionRule, difficulty string) []esqdomain.QuestionRule` and HTTP rule responses without `difficulty`.

- [ ] Write tests proving the exam-set difficulty replaces empty and stale rule difficulty.
- [ ] Run `go test ./internal/examsetquestion/usecase -run ExamSetDifficulty -count=1` and confirm failure because the helper is missing.
- [ ] Normalize rules in save, capacity, bulk-add, auto-assign, and reorder flows; omit rule difficulty from request/response parsing.
- [ ] Run `go test ./internal/examsetquestion/usecase -count=1` and confirm success.

### Task 2: Admin Blueprint Experience

**Files:**
- Modify: `web/src/lib/api/admin/exam-set-questions.ts`
- Modify: `web/src/components/admin/exam-set-questions/AutoAssignPanel.tsx`
- Modify: `web/src/components/admin/exam-set-questions/QuestionBankPanel.tsx`
- Modify: `web/src/app/(admin)/admin/exam-sets/[id]/questions/page.tsx`

**Interfaces:**
- Consumes: `AdminExamSet.difficulty` and `AutoAssignRule` containing only `subject_id`, optional `tag_id`, and `count`.
- Produces: blueprint editor copy, read-only difficulty badge, and automatic/manual assignment actions governed by saved blueprint.

- [ ] Remove rule-level difficulty from frontend types, draft keys, capacity requests, and matching logic.
- [ ] Pass exam-set difficulty to the blueprint editor and show its Thai label.
- [ ] Replace condition-oriented labels, statuses, errors, and toasts with blueprint-oriented copy.
- [ ] Label the two assignment paths as `สุ่มคำถามอัตโนมัติ` and `เลือกคำถามเอง`.
- [ ] Run `npm run lint` and fix new errors.

### Task 3: Regression Verification

**Files:**
- Verify all files modified in Tasks 1-2.

**Interfaces:**
- Consumes: completed backend and frontend behavior.
- Produces: verified production-ready change.

- [ ] Run `go test ./...` and confirm zero failures.
- [ ] Run `npm run build` and confirm exit code 0.
- [ ] Run `git diff --check` for touched files and inspect the final diff for unrelated edits.
