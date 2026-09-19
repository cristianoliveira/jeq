---
id: TASK-0017
title: Emit structured unknown-command diagnostics
status: done
depends_on: [TASK-0002, TASK-0008]
priority: high
tags: []
---

# Emit structured unknown-command diagnostics

## Problem
Unknown commands currently exit 2 with empty stdout and stderr, so an agent cannot discover the valid alternative or recover.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Unknown command and unknown flag exit 2 and emit exactly one structured error document to stdout with one trailing newline.
- [ ] Document carries `GEV_INPUT_INVALID`, a safe message, and a recovery value naming valid commands/flags or the closest deterministic alternative.
- [ ] No raw Cobra prose or duplicate diagnostics leak to stderr; help remains exit 0.
- [ ] Renderer is injected; cli does not import infra/render (ADR 0002 arrow stays green).
- [ ] Golden tests cover unknown command, unknown flag, misspelled command suggestion, and help.

## Notes
Found by Kelly as F-1 in `docs/qa/d0-verification.md`. Depends on the JSON renderer so the fix satisfies ADR 0001 instead of adding temporary plain-text stderr output.

