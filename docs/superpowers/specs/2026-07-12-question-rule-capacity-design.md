# Question Rule Capacity Design

## Goal

Prevent an exam-set question rule from requesting more questions than the question bank can provide for that rule's subject, optional tag, and optional difficulty.

## Capacity Definition

Only questions that are published and active are eligible.

For each rule, return:

- `total_available`: all eligible bank questions matching the rule, including questions already assigned to this exam set.
- `assigned_count`: matching questions already assigned to this exam set.
- `remaining_available`: `total_available - assigned_count`.

The rule's maximum target count is `total_available`, because the rule count describes the final total inside the exam set rather than only newly added questions.

## API Contract

Add a capacity endpoint under the existing exam-set question-rule routes. It accepts draft rules and returns one capacity result per submitted rule in the same order.

Rule saving performs the same count directly on the server. If `count > total_available`, saving fails with a validation error that identifies the rule and its maximum available count. The server remains authoritative if the bank changes after the capacity preview.

## Repository Query

Count published, active questions by:

- Required `subject_id`.
- Optional membership in `question_tag_mappings` for `tag_id`.
- Optional exact `difficulty`.

Assigned count uses the same criteria and joins `exam_set_questions` for the current exam set.

## UI Behavior

Whenever a rule's subject, tag, or difficulty changes, refresh capacity for all draft rules using one debounced request.

Each rule row displays:

`มีทั้งหมด N · อยู่ในชุดแล้ว A · เพิ่มได้อีก R`

The numeric input receives `max=N`. When the entered count exceeds the maximum, show an inline error and disable saving and automatic assignment. Capacity loading must not erase the user's draft values.

If no matching questions exist, show `ไม่มีคำถามที่ตรงเงื่อนไข` and keep the rule invalid until its conditions change.

## Error Handling

- Invalid subject/tag identifiers continue to use existing validation errors.
- Capacity lookup failures show a non-destructive inline retry state and keep save disabled.
- Server-side save validation returns the authoritative maximum in the error message.

## Verification

- Repository tests cover subject-only, tag, difficulty, published/active, and assigned counts.
- Use-case tests reject rule counts above capacity and accept counts equal to capacity.
- Frontend lint and production build pass.
- Manual browser verification checks capacity copy, numeric maximum, and disabled save state.
