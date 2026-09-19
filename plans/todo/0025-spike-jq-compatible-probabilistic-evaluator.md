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

## Candidate algebra and interface

```text
probabilistic map:       N values -> N enriched values     (N independent calls)
probabilistic aggregate: N values -> 1 enriched aggregate (1 bounded call)
probabilistic fold:      N values -> 1 evolving result    (N dependent calls)
```

Map already exists. The spike must distinguish a one-call aggregate from a true fold: folds are order-sensitive, expensive, and compound uncertainty, so they must be explicit and budgeted rather than the default meaning of `reduce`.

```text
gev eval '<jq-compatible program>'

gev::judge(name; {state, questions})
  current object -> current object + _gev.<name>
```

`gev::judge` is the only effectful builtin. It resolves the model from gev configuration unless explicitly overridden, strictly validates one TypeSafe request, performs one logical evaluation, and appends the complete response evidence through the TASK-0023 engine. Traversal, construction, jq `map`/`reduce`, `select`, variables, and branching remain jq semantics rather than gev inventions.

A dedicated `gev reduce` may be justified as a lower-level peer to `gev map`: bounded JSON/NDJSON records become one envelope whose state is the input array, followed by one judgment. The spike must compare this with `jq -s | gev map` instead of assuming another command is necessary.

## Acceptance criteria
- [ ] A decision record compares external jq pipelines, embedding `gojq`, native jq extension mechanisms, and writing a new language. It records compatibility gaps, license/supply-chain/maintenance cost, and rejects any option that silently claims full jq compatibility.
- [ ] An isolated, non-production prototype proves or disproves a custom `gev::judge(name; request)` function over one JSON value. It uses the TASK-0023 append-only engine; it does not add a shipped root command or production dependency.
- [ ] The prototype demonstrates all three semantics separately: judgment inside jq `map`, deterministic jq `reduce` followed by one judgment, and judgment inside a true fold. Evidence states exact calls/order/cost and recommends which forms deserve supported syntax.
- [ ] Effect semantics are explicit and tested: only the evaluated branch calls AI; each invocation is sequential; jq fan-out/backtracking can cause multiple calls; collisions and invalid requests fail before network; operational errors abort; model output remains inert JSON.
- [ ] The design specifies mandatory call, output-count, output-byte, input-byte, program-byte, collection-size, and wall-clock bounds. It investigates cancellation of recursion/long-running filters and rejects an interpreter that cannot be reliably bounded.
- [ ] Model selection is not required in normal programs. The study defines transparent precedence among explicit override, environment, user/explicit-file configuration, and stable built-in default; records the resolved model in every judgment; and prevents an untrusted repository config from silently changing credentials or API endpoints.
- [ ] The design identifies nondeterministic or ambient jq features (`env`, time, input streams, imports/modules, debug/stderr, filesystem access where applicable) and either disables them or explains why they cannot violate the deterministic/sandbox boundary.
- [ ] Every judgment retains model, answers, and usage under `_gev.<name>`. The study explains how aggregate cost remains observable if later jq filters project evidence away; no hidden call is treated as free.
- [ ] Support, release, and incident examples run through the prototype with deterministic fake responses. Evidence compares program size/readability, exact request count/order, output shape, failure locality, and privacy projection against current pipelines.
- [ ] Tests cover a false branch (zero calls), one call, mapped fan-out calls, one-call aggregate, dependent fold, name collision, malformed request, unknown catalog key, evaluator failure, call-budget exhaustion, collection/output-budget exhaustion, cancellation, configured-model resolution, and attempted access to forbidden ambient features.
- [ ] The final report gives a clear go/no-go recommendation and the smallest next task. It distinguishes jq-compatible syntax from exact jq behavior and does not market experimental behavior as supported CLI.

## Constraints
- Do not design another traversal, expression, condition, or template language.
- Do not add actions, shell execution, model-selected code/paths, implicit concurrency, speculative evaluation, or automatic evidence removal.
- JSON remains the interoperability format under existing ADRs; this spike does not reopen renderer format decisions.
- `map` and `gate` remain stable lower-level primitives regardless of the result.

## Notes
Prefer a temporary standalone module for dependency experiments. Commit durable findings and fixtures, not throwaway binaries, dependency changes, or `.tmp` artifacts.

