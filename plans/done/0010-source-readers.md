---
id: TASK-0010
title: Source readers
status: done
depends_on: [TASK-0005]
priority: normal
tags: []
---

# Source readers

## Problem
Explicit file and stdin sources must never block on a TTY or read beyond the requested input.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Read files only for explicitly selected flags; `-` means explicit stdin for `--request`, `--questions`, `--state-file`, and `--state-json`.
- [ ] Never read stdin implicitly. Explicit `-` on a terminal fails pre-read with JEQ_INPUT_INVALID rather than blocking.
- [ ] Reject configurations requiring stdin twice (for example `--questions - --state-json -`) before any read.
- [ ] Enforce an injected byte limit with deterministic oversize failure; preserve file/stdin bytes exactly within the limit.
- [ ] Not-found, unreadable, directory, empty where forbidden, TTY, and oversize failures include the offending source and actionable recovery without leaking paths beyond the supplied value.
- [ ] Table tests use temp files and injected readers/TTY detector; assert exact read counts and zero reads for conflicts.

## Notes
Filesystem/stdin access stays in infra/cli. Domain Compose receives resolved bytes and remains pure.

