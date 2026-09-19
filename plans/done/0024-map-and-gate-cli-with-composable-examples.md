---
id: TASK-0024
title: Map and gate CLI with composable examples
status: done
depends_on: [TASK-0023]
priority: high
tags: []
---

# Map and gate CLI with composable examples

## Problem
Users need readable Unix primitives, not hidden orchestration. They must enrich JSON streams, inspect intermediate evidence, apply explicit offline probability policy, and continue piping the same records.

## Desired outcome
`gev map` performs one named semantic enrichment per record; `gev gate` performs one named offline numeric policy decision. `jq` and the shell remain the branching and orchestration language.

## Acceptance criteria
- [ ] `gev map --as <name>` accepts JSON or NDJSON, sequentially enriches each object under `_gev.<name>`, preserves order, and emits the same framing. Exactly one mode is required: composed `--questions <file> [--state-pointer <RFC6901>] [--model]`, or native `--request-pointer <RFC6901>` for a strictly validated request already constructed in the record by ordinary tools such as `jq`.
- [ ] Map validates flags/questions/input before client construction where possible; enforces bounded record size/count; defaults to fail-fast while preserving already-emitted records; each valid record causes exactly one request and no automatic replay of ambiguous failures.
- [ ] `gev gate --as <name> --value-pointer <RFC6901> --pass-min <0..1> --reject-max <0..1>` is offline, appends pass/reject/uncertain evidence, and supports JSON/NDJSON. Aggregate exit is 10 if any reject, else 11 if any uncertain, else 0; operational/usage/interruption stay 1/2/130.
- [ ] Gate rejects overlapping/invalid thresholds and nonnumeric/out-of-range values before emitting a misleading decision. Existing names are never overwritten.
- [ ] Help is concise and copyable. Commands emit structured JSON/NDJSON only; stderr is diagnostics; no prompts, shell execution, hidden state, or model-derived actions.
- [ ] Compiled-binary black-box tests cover JSON/NDJSON, two chained maps, map-to-gate, escaped/missing pointers, collisions, duplicate keys, limits, partial-output fail-fast, exact request counts, 0/10/11 aggregation, 1/2/130 propagation, newline/stderr/redaction, and interruption.
- [ ] Replace Python subprocess examples/adapter with readable `map | jq | map | gate` pipelines and fixtures; remove Python from the dev shell if no other use remains. Keep Bash examples labeled low-level.
- [ ] Full gate, govulncheck, architecture checks, and opt-in live synthetic pipelines pass; record requests/tokens/cost without logging credentials.

## Non-goals
- No workflow manifests, branching DSL, actions, remote references, output templates, concurrency, DAGs, loops, or persistent state.

## Notes
A future costly-workflow optimizer is acceptable only if it compiles to these same observable envelope transformations and can expose checkpoints.

