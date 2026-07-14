# Exam Question Rule Order Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist rule order and keep assigned questions grouped in that order after rule changes, automatic assignment, and manual assignment.

**Architecture:** `exam_set_question_rules.rule_order` is the authoritative group order. The use case performs a stable regroup by matching each assigned question to one rule, sorting by rule order and existing question number, then calling the transactional repository reorder. The frontend prevents move controls from crossing rule-group boundaries.

**Tech Stack:** Go, GORM, PostgreSQL, Echo, Next.js 16, React, TypeScript.

## Global Constraints

- Preserve existing question order inside each rule group.
- Automatic and manual assignment use the same regrouping path.
- Reject manual reorder requests that cross rule boundaries.
- Do not alter unrelated dirty-worktree changes.

---

### Task 1: Persist Rule Order

**Files:**
- Modify: `migrations/000022_exam_set_question_rules.up.sql`
- Modify: `internal/examsetquestion/domain/types.go`
- Modify: `internal/examsetquestion/repository/postgres.go`
- Test: `internal/examsetquestion/repository/postgres_test.go`

**Interfaces:**
- Produces: `QuestionRule.Order int` and ordered `GetQuestionRules(ctx, examSetID)` results.

- [ ] **Step 1: Write the failing repository test**

Create rules in submitted order 2, 1, 3 and assert `GetQuestionRules` returns `Order` values `1, 2, 3` after replacement.

- [ ] **Step 2: Run the focused test and verify failure**

Run: `go test ./internal/examsetquestion/repository -run TestReplaceQuestionRulesPreservesOrder -v`

Expected: FAIL because `QuestionRule` has no persisted order.

- [ ] **Step 3: Add the order field**

Add `rule_order INT NOT NULL DEFAULT 1` to the SQL migration and model. Set `Order: index + 1` in `ReplaceQuestionRules`, map it to the domain, and query with `ORDER BY rule_order ASC, id ASC`.

- [ ] **Step 4: Run the focused test**

Run: `go test ./internal/examsetquestion/repository -run TestReplaceQuestionRulesPreservesOrder -v`

Expected: PASS.

### Task 2: Stable Automatic Regrouping

**Files:**
- Modify: `internal/examsetquestion/usecase/usecase.go`
- Test: `internal/examsetquestion/usecase/usecase_test.go`

**Interfaces:**
- Consumes: ordered `[]domain.QuestionRule`.
- Produces: `regroupQuestionsByRules(ctx context.Context, examSetID uuid.UUID, rules []domain.QuestionRule) error`.

- [ ] **Step 1: Write failing regrouping tests**

Cover mixed assigned questions `B2, A2, B1, A1` and assert regrouped order `A2, A1, B2, B1`. Add a second test that reverses rule order and preserves order inside each group.

- [ ] **Step 2: Run focused use-case tests**

Run: `go test ./internal/examsetquestion/usecase -run 'TestRegroupQuestionsByRules|TestSetQuestionRulesRegroupsExistingQuestions' -v`

Expected: FAIL because regrouping is not implemented.

- [ ] **Step 3: Implement stable regrouping**

Load all assigned questions, resolve their full question records, find each matching rule with `matchingRuleIndex`, and stable-sort by rule index while using existing `question_no` as the secondary key. Send sequential `domain.ReorderItem` values to `repo.Reorder`.

- [ ] **Step 4: Invoke regrouping at all trigger points**

Call regrouping after `ReplaceQuestionRules` and after successful `repo.BulkAdd`. Auto assignment inherits the behavior through `BulkAdd`.

- [ ] **Step 5: Run use-case and full backend tests**

Run: `go test ./internal/examsetquestion/... && go test ./...`

Expected: PASS.

### Task 3: Restrict Manual Reordering to Rule Groups

**Files:**
- Modify: `internal/examsetquestion/usecase/usecase.go`
- Modify: `web/src/lib/api/admin/exam-set-questions.ts`
- Modify: `web/src/app/(admin)/admin/exam-sets/[id]/questions/page.tsx`
- Modify: `web/src/components/admin/exam-set-questions/AssignedQuestionsPanel.tsx`
- Test: `internal/examsetquestion/usecase/usecase_test.go`

**Interfaces:**
- Backend `Reorder` accepts only sequences whose rule-index sequence remains nondecreasing.
- Frontend computes a rule key for every assigned question and disables boundary-crossing move controls.

- [ ] **Step 1: Write the failing cross-group reorder test**

Submit a reorder that places a rule-2 question before a rule-1 question and assert `ErrQuestionOutsideExamSetRules`.

- [ ] **Step 2: Implement backend validation**

Load ordered rules and requested questions, map each question to a rule index, and reject the request when any later rule index is smaller than the previous one. Preserve existing validation for complete sequential numbering.

- [ ] **Step 3: Expose assigned question tags needed for matching**

Add `tags` to `AssignedExamQuestion` and the assigned-question API response so the frontend can apply the same subject/tag/difficulty matching contract.

- [ ] **Step 4: Disable invalid move controls**

Pass rule-boundary metadata into `AssignedQuestionsPanel`. Disable move-up at the first item of a rule group and move-down at the last item of a rule group.

- [ ] **Step 5: Verify all surfaces**

Run: `go test ./...`

Run: `npm run lint && npm run build`

Expected: backend tests pass; frontend has no lint errors; production build and SEO verification pass.
