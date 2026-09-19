---
id: TASK-0025
title: Spike jq-compatible probabilistic evaluator
status: todo
depends_on: [TASK-0024]
priority: normal
tags: [exploration]
---

# Spike jq-compatible probabilistic evaluator

## Problem
Shell pipelines remain verbose when probabilistic judgments appear inside traversal, branching, mapping, or catalog construction. Adding those programming constructs directly to gev would recreate jq badly, while native jq cannot call the TypeSafe evaluator as a custom effect.

## Desired outcome
We know whether an embedded jq-compatible evaluator can safely add one explicit probabilistic effect without hiding calls, costs, evidence, or failures. A go decision produces a separate implementation task; a no-go decision leaves `map | jq | gate` as the supported interface.

## Candidate interface

```text
gev eval '<jq-compatible program>'

gev::judge(name; native_request)
  current object -> current object + _gev.<name>
```

`gev::judge` is the only effectful builtin. It strictly validates one native TypeSafe request, performs one logical evaluation, and appends the complete response evidence through the TASK-0023 engine. Traversal, construction, `map`, `select`, variables, and branching remain jq semantics rather than gev inventions.

## Acceptance criteria
- [ ] A decision record compares external jq pipelines, embedding `gojq`, native jq extension mechanisms, and writing a new language. It records compatibility gaps, license/supply-chain/maintenance cost, and rejects any option that silently claims full jq compatibility.
- [ ] An isolated, non-production prototype proves or disproves a custom `gev::judge(name; request)` function over one JSON value. It uses `pipeline.EnrichRequest`; it does not add a shipped root command or production dependency.
- [ ] Effect semantics are explicit and tested: only the evaluated branch calls AI; each invocation is sequential; jq fan-out/backtracking can cause multiple calls; collisions and invalid requests fail before network; operational errors abort; model output remains inert JSON.
- [ ] The design specifies mandatory call, output-count, output-byte, input-byte, program-byte, and wall-clock bounds. It investigates cancellation of recursion/long-running filters and rejects an interpreter that cannot be reliably bounded.
- [ ] The design identifies nondeterministic or ambient jq features (`env`, time, input streams, imports/modules, debug/stderr, filesystem access where applicable) and either disables them or explains why they cannot violate the deterministic/sandbox boundary.
- [ ] Every judgment retains model, answers, and usage under `_gev.<name>`. The study explains how aggregate cost remains observable if later jq filters project evidence away; no hidden call is treated as free.
- [ ] Support, release, and incident examples run through the prototype with deterministic fake responses. Evidence compares program size/readability, exact request count/order, output shape, failure locality, and privacy projection against current pipelines.
- [ ] Tests cover a false branch (zero calls), one call, fan-out calls, name collision, malformed request, unknown catalog key, evaluator failure, call-budget exhaustion, output-budget exhaustion, cancellation, and attempted access to forbidden ambient features.
- [ ] The final report gives a clear go/no-go recommendation and the smallest next task. It distinguishes jq-compatible syntax from exact jq behavior and does not market experimental behavior as supported CLI.

## Constraints
- Do not design another traversal, expression, condition, or template language.
- Do not add actions, shell execution, model-selected code/paths, implicit concurrency, speculative evaluation, or automatic evidence removal.
- JSON remains the interoperability format under existing ADRs; this spike does not reopen renderer format decisions.
- `map` and `gate` remain stable lower-level primitives regardless of the result.

## Notes
Prefer a temporary standalone module for dependency experiments. Commit durable findings and fixtures, not throwaway binaries, dependency changes, or `.tmp` artifacts.

