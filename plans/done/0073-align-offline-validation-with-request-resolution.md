---
id: TASK-0073
title: Align offline validation with request resolution
status: done
depends_on: []
priority: high
tags: [cli, ux, validation, configuration, stateless]
---

# Align offline validation with request resolution

## Problem
With `JEQ_DEFAULT_MODEL=chosen-model`, composed validate reports `jev-latest` while ask sends `chosen-model`. validate uses the legacy resolver rather than the configured resolver. A user cannot trust the local check to describe the next request. Native requests also accept a `--model` flag that is silently ignored.

## Desired outcome
Given the same current inputs, composed validation describes the model that execution will send. Users can inspect model selection and its source offline, without authentication or changing future invocations.

## Approach
Share credential-free request/model resolution between validation and execution. Resolve current flags, environment, and existing read-only user config each time. Keep credential lookup and client creation outside validation. Extend the existing plain validation receipt rather than adding a persistent configuration manager.

## Acceptance criteria
- [x] Composed validate and ask resolve the same outgoing request model for explicit flags, `JEQ_DEFAULT_MODEL`, named-provider defaults, legacy defaults, and provider fallback defaults.
- [x] Validation states the resolved model and its source in plain text so the question is answered in one invocation with valid inputs.
- [x] Validation works with absent credentials and makes zero client or network calls. It does not use `models` to resolve or confirm a model.
- [x] Explicit malformed/missing config fails consistently before authentication. An absent optional config keeps the documented fallback behavior.
- [x] Native requests remain authoritative and independent of composed-model config. Reject an explicitly supplied `--model` alongside native `--request` before I/O instead of silently ignoring it; document how to change the request document itself.
- [x] Tests compare the validation result with the model in the request captured by a local fake evaluator, not the response model version.
- [x] Run two fresh invocations with different environment/config inputs and prove the second uses only its own inputs. Assert that discovery and validation leave config and isolated HOME contents unchanged.
- [x] Help documents the precedence and the distinction between local validation and provider support for a model.
- [x] Focused tests and the configured watcher gate pass.

## Constraints
Statelessness is mandatory. Existing user configuration is read-only input, not operational memory. No `config set`, login store, last-used provider/model, session, cache, or automatic file creation. Do not expose credentials or raw state in the receipt.

## Context
Inspect `internal/cli/validate.go:runValidate`, `model.go`, `config.go`, `ask.go:runAsk`, and `docs/ARCHITECTURE.md`'s statelessness contract. This completes the shared-resolution intent of TASK-0052 without adding state.

## Completion
- Code commit: `f37fc4e fix(validate): share configured model resolution (TASK-0073)`.
- QA: Kelly approved exact HEAD `f37fc4e02d8e6038725983de29ecd6cdc801c065`; no blocking gaps.
- Verification: `go test ./internal/cli ./internal/domain/jeq -count=1`; watcher generation 110 passed the configured full gate.
- No paid or network calls. Task moved to done only after independent QA.
