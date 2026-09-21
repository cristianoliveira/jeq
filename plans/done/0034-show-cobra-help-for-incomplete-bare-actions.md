---
id: TASK-0034
title: Show Cobra help for incomplete bare actions
status: done
depends_on: []
priority: high
tags: []
---

# Show Cobra help for incomplete bare actions

## Problem
Bare action commands such as jeq ask currently enter domain validation and emit an input error. A command with no invocation intent should show its native Cobra help; actual malformed invocations should still fail.

## Acceptance criteria
- [x] Bare `jeq ask`, `validate`, `map`, `reduce`, and `gate` are byte-identical to their respective `--help`, exit 0, write stdout only, and perform no environment/config/stdin/file/client/network/renderer work.
- [x] Preserve real invocation validation: once any action-specific flag is supplied, incomplete/conflicting input uses standard Cobra stderr/domain code and the established exit class.
- [x] Preserve commands whose bare invocation is a complete action: `jeq version` prints version; `jeq models` lists models or reports operational/auth failure; `jeq examples` and recipe leaves use native Cobra help; completion keeps Cobra behavior.
- [x] Audit every command's bare/help/success/failure surface in one table and add a parameterized black-box regression so no other custom no-arg behavior remains.
- [x] Root and examples TASK-0031/0032 behavior and all jeq rebrand contracts remain unchanged.
- [x] Update current docs/QA, run full checks/govulncheck/independent QA, and commit.

## Constraint
Use Cobra help directly; do not create another dashboard, custom usage renderer, or duplicated help text.

