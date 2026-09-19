---
id: TASK-0004
title: Contract validation rules
status: done
depends_on: [TASK-0003]
priority: normal
tags: []
---

# Contract validation rules

## Problem
Invalid documents must fail locally, before credentials or network, with actionable stable codes.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Local rules are exactly the client-owned invariants (OQ-2 resolved): valid UTF-8 JSON; no duplicate keys; non-empty state; at least one question; known `type`; non-empty `instructions`; choice `criteria` non-empty map; score `criteria` ≥ 2 levels; noul `criteria` (if present) an object of true/false descriptions.
- [ ] Every rule outside that list defers to the server; a 422 classifies as exit 1 surfacing the server's field details.
- [ ] Each rule has a failing fixture + registered stable code (`JEQ_REQUEST_INVALID`) + actionable field path and recovery text.
- [ ] All checks run pre-credential and pre-network (fake server must count 0 requests; no env lookup).
- [ ] Meta-test: every failure fixture maps to a registered stable code.

## Notes
Resolves Kelly OQ-2. Server is the semantic authority; jeq only rejects what it can check without the server.

