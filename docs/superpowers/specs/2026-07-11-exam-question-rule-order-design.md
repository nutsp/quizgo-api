# Exam Question Rule Order Design

## Goal

Questions in an exam set must be grouped in the same order as the saved question-selection rules. This ordering applies consistently after rule edits, automatic assignment, and manual assignment.

## Data Model

Add `rule_order` to `exam_set_question_rules` as a positive integer. Rule replacement writes `rule_order` from the submitted array index, starting at 1. Rule reads order by `rule_order`, with `id` only as a deterministic fallback.

The SQL migration and GORM model must both include this field so existing local databases receive it through AutoMigrate and deployed databases receive it through SQL migration.

## Ordering Contract

Each assigned question is matched to exactly one saved rule using the existing subject, tag, and difficulty matching logic. Rules are already validated to prevent overlapping conditions.

Questions are sorted by:

1. The matching rule's `rule_order`.
2. Their existing `question_no` within that rule.

This is a stable regrouping operation: moving a rule changes the position of its question group but does not scramble questions inside the group.

## Trigger Points

Automatic regrouping runs after:

- Saving or replacing question-selection rules.
- Adding questions manually through bulk add or single add.
- Automatic assignment, through the same bulk-add path.

Removing a question continues to renumber the remaining questions without changing their relative order.

## Manual Reordering

Manual drag or move controls may reorder questions only within the same rule group. A reorder request that moves a question across rule boundaries is rejected by the API. This keeps the saved rule order authoritative.

## Failure Handling

- Saving rules is rejected if an existing assigned question does not match any rule.
- Regrouping is rejected if a question cannot be matched to a rule.
- Database reorder runs transactionally so partial question numbering is not persisted.

## UI Behavior

The assigned-question list follows rule order automatically. Rule groups should remain visually identifiable by subject/group/difficulty labels. Move controls are disabled at the first and last question within each rule group so the UI does not offer an invalid cross-group move.

## Verification

- Repository test: rules persist and load by `rule_order`.
- Use-case test: mixed questions are stably grouped by rule order.
- Use-case test: changing rule order regroups existing questions.
- Use-case test: cross-rule manual reorder is rejected.
- Build and lint the frontend after updating move-control boundaries.
