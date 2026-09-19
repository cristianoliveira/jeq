---
id: TASK-0013
title: validate command
status: done
depends_on: [TASK-0004, TASK-0005, TASK-0020]
priority: normal
tags: []
---

# validate command

## Problem
Agents need offline document checking to avoid paid 422 round trips.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] `jeq validate` accepts the same native/composed source flags and conflict matrix as `ask`, including explicit stdin and byte bounds.
- [ ] It performs decode, composition, and the closed local validation set only—no credential lookup, HTTP client call, retry, or paid request.
- [ ] Success exits 0 with a deterministic structured receipt: valid=true, mode, resolved model, and question count; it does not echo state content.
- [ ] Failure uses the selected structured error document, stable code/recovery, and exit 2; exactly one document/newline.
- [ ] Tests assert zero env/network access, native/composed happy paths, every validation fixture, conflicts, stdin, renderer selection, and state non-disclosure.

## Notes
This command prevents avoidable paid 422s but does not claim server-semantic validity.

