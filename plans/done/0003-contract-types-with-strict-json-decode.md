---
id: TASK-0003
title: Contract types with strict JSON decode
status: done
depends_on: [TASK-0001]
priority: high
tags: []
---

# Contract types with strict JSON decode

## Problem
jeq must accept the full System One document without losing unknown server fields and without duplicate-key ambiguity.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Decodes Request, Question (noul/choice/score), Response, Answer, Usage, Models.
- [ ] Duplicate JSON keys rejected (not last-wins); invalid JSON and non-UTF-8 rejected.
- [ ] Unknown fields preserved byte-semantically through decode→encode in BOTH native and composed modes (OQ-1 resolved: passthrough wins; strictness applies to duplicate keys and known-field types only — unknown fields never rejected).
- [ ] Known-field type mismatches produce stable codes naming the field path.
- [ ] Response decode is tolerant: unknown server fields survive; missing known fields classify as JEQ_RESPONSE_INVALID.
- [ ] Round-trip property tests: fixture documents decode→encode to semantically identical JSON.

## Notes
Resolves Kelly OQ-1. Update docs/qa/acceptance-d0-d2.md D1-3/D2-19 wording accordingly (unknown field → passthrough, not exit 2).

