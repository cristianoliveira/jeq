---
id: TASK-0021
title: Executable Unix decision workflow examples
status: done
depends_on: []
priority: high
tags: []
---

# Executable Unix decision workflow examples

## Problem
Users can call jeq, but there are no complete pipe-oriented workflows that demonstrate routing, policy gating, and stream ranking safely or provide executable evidence for designing future workflow primitives.

## Desired outcome
A user can clone the repository and run small shell workflows that treat `jeq` as a semantic Unix filter while keeping branching, thresholds, and execution explicit in deterministic code.

## Acceptance criteria
- [ ] `examples/support-routing` reads a ticket on stdin, asks route/urgency/escalation together, and emits one JSON decision receipt with a closed allowlisted action.
- [ ] `examples/change-risk-gate` reads a diff on stdin and emits one JSON policy receipt; exit `0` means pass, `10` review/block, `11` uncertain, while jeq operational/usage/interruption exits remain distinguishable.
- [ ] `examples/issue-ranking` reads NDJSON, evaluates each record, preserves an input ID, and emits correlated NDJSON suitable for `jq`, with deterministic order and explicit per-record failure policy.
- [ ] Scripts use strict shell mode, accept an injected `JEQ_BIN` and `JEQ_BASE_URL`, never evaluate model-produced shell, never echo credentials, and document paid-call/cost behavior.
- [ ] Question specifications and representative fixtures are versioned beside each workflow.
- [ ] Compiled-binary E2E tests use a deterministic local fake TypeSafe server, assert requests and outputs/exit statuses, and make zero production calls.
- [ ] `examples/README.md` explains composition, confidence/probability policy, opt-in live usage, limitations, and evidence that should shape future `jeq gate`/`jeq map` primitives rather than a workflow DSL.
- [ ] `jq` is available in the Nix dev shell and the normal gate runs all example tests.

## Non-goals
- No workflow DSL, arbitrary command execution, production side effects, or new networked CLI subcommand.
- No claim that demonstration thresholds are calibrated for real deployment.

## Notes
Start with tests. Prefer shell and `jq` as the orchestration language; keep jeq focused on typed judgments.

