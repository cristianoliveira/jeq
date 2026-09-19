---
id: TASK-0022
title: Readable multi-stage decision workflow examples
status: done
depends_on: []
priority: high
tags: []
---

# Readable multi-stage decision workflow examples

## Problem
The Bash examples prove Unix composition but obscure the decision story. Users need readable examples that demonstrate deterministic rules, confidence-aware policy, and justified multi-stage Jev calls before a workflow DSL can be designed responsibly.

## Desired outcome
Users can understand the decision policy from top-to-bottom without first understanding defensive shell mechanics, while stdin/stdout/exit contracts remain Unix-composable.

## Acceptance criteria
- [ ] Add a small standard-library Python `jeq` subprocess adapter that preserves structured output, stderr, and exit status without becoming an orchestration DSL.
- [ ] A simple support-routing Python example reads one ticket from stdin and clearly separates judgment, confidence policy, and allowlisted action selection.
- [ ] A release-readiness example applies deterministic hard blockers before any paid call, then combines typed semantic judgments with explicit pass/canary/review/block policy and an auditable decision trace.
- [ ] An incident-triage cascade uses stage-one category/severity judgments, stops for human review when uncertain, and only then makes a justified second call to select from category-specific allowlisted runbooks.
- [ ] Every successful receipt preserves model, usage, confidence/probability evidence, stage count, and policy reason without echoing raw input or credentials.
- [ ] Operational jeq failures retain their structured document and 1/2/130 status; policy outcomes use documented non-conflicting statuses.
- [ ] Examples use type hints, named data structures/functions, early returns, and comments that explain why—not shell plumbing.
- [ ] Root and per-example docs use progressive disclosure, diagrams, copy-paste commands, paid-call counts, cost warnings, and explicit non-calibrated thresholds.
- [ ] Deterministic compiled-binary E2E tests cover zero-call hard blockers, one-stage uncertainty, two-stage happy path, candidate allowlisting, request bodies/counts, decision branches, and error propagation with no production network.
- [ ] Python is available in the Nix dev shell and the normal gate runs all tests.

## Non-goals
- No workflow DSL, Python package, direct TypeSafe HTTP client, arbitrary command execution, or production side effects.
- Do not hide probabilities behind unexplained booleans or claim demonstration thresholds are calibrated.

## Notes
Keep Bash examples as low-level references. Make Python the recommended learning path. Test behavior before implementation.

