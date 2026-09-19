---
id: TASK-0032
title: Make all CLI discovery native Cobra
status: todo
depends_on: [TASK-0033]
priority: high
tags: []
---

# Make all CLI discovery native Cobra

## Problem
The examples command still renders a custom catalog and recipe dashboard. Audit every command surface and use Cobra help for navigation/documentation while reserving command output for actual version, model, validation, and evaluation results.

## Acceptance criteria
- [ ] Audit and record every root/subcommand no-arg, help, success, and failure output; classify it as navigation/documentation, human result, machine result, or diagnostic.
- [ ] Navigation/documentation uses native Cobra command trees and help only. No custom catalog, dashboard, `Next:`, manually formatted usage, or duplicate help path.
- [ ] Make each workflow recipe a real child of `gev examples`. Bare `gev examples` equals `gev examples --help`; each bare recipe equals its `--help`. Parent lists recipes through Cobra `Available Commands`.
- [ ] Recipe purpose/requirements/cost/shapes/privacy/exits live in Cobra Long; runnable shell lives in Cobra Example. Preserve all five recipes and offline discovery.
- [ ] Unknown recipe, extra argument, unknown flag, and help use native Cobra wording/channel/status. No custom recipe error or recovery formatter.
- [ ] Actual human results remain concise plain stdout: version, model list, validate receipt. Actual machine results remain JSON/NDJSON: ask, map, reduce, gate. Diagnostics/errors remain Cobra stderr.
- [ ] Remove obsolete catalog/summary/next-step types and human formatting functions. Keep recipe definitions as one typed source.
- [ ] Update current docs/tests, verify all command surfaces from a built binary, run full checks/govulncheck/architecture/independent QA, and commit.

## Non-goals
Do not turn actual command results into help text, restore JSON errors, add color/TTY behavior, or add interactive selection.

