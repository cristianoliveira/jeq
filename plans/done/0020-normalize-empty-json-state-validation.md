---
id: TASK-0020
title: Normalize empty JSON state validation
status: done
depends_on: [TASK-0004]
priority: normal
tags: []
---

# Normalize empty JSON state validation

## Problem
Pretty-printed empty object or array state passes local validation while compact empty state is rejected, making equivalent JSON depend on whitespace.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Semantically empty objects and arrays are rejected regardless of whitespace (`{}`, `{\n}`, `[]`, `[\n]`).
- [ ] Non-empty object/array state remains accepted; string-state behavior is unchanged.
- [ ] Validation compares parsed JSON shape, not formatting or byte length.
- [ ] Table tests cover compact, pretty-printed, nested-empty-but-non-empty, and non-empty states.

## Notes
QA finding F-D1-1 in `docs/qa/d1-verification.md`. Must land before `ask` and `validate` acceptance.

