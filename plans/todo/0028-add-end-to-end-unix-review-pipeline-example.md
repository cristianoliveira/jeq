---
id: TASK-0028
title: Add end-to-end Unix review pipeline example
status: doing
depends_on: [TASK-0027]
priority: normal
tags: [examples, unix]
---

# Add end-to-end Unix review pipeline example

## Problem
The aggregate code-smell example demonstrates reduce behind one wrapper, but it does not visibly show gev records flowing through Unix pipes or compose map, jq, reduce, and gate in one workflow. Users cannot see how the CLI primitives form a transparent multi-stage pipeline.

## Desired outcome
A new, separate executable example makes the JSON stream and every stage visible: shell emits one record per file, `gev map` adds a local judgment per record, `jq` shapes the intermediate record, `gev reduce` evaluates the complete related collection once, `gev gate` applies offline policy, and final `jq` removes source before output.

## Acceptance criteria
- [ ] Keep `examples/code-smell-review` unchanged. Add a separate `examples/unix-review-pipeline` example with an executable Bash script, question files, small fixtures, and README.
- [ ] The main script is one readable `set -euo pipefail` pipeline: record emitter → `gev map --input ndjson` → `jq -c` projection → `gev reduce --input ndjson` → `gev gate` → final `jq` safe projection. No hidden workflow interpreter, temp source bundle, `eval`, model-derived shell/path, or generated explanation.
- [ ] Preflight all input paths before the first pipe/API call. Preserve argument order; accept 1..20 readable regular files, max 256 KiB each; check required binaries. Use Bash 3.2-compatible syntax.
- [ ] Map makes one local positive Noul judgment per file using `/file` state. Reduce makes one positive Noul judgment across the complete mapped collection, where each item has `file:{path,content}` and a numeric local signal. Questions state the exact shape, treat source/comments/strings as untrusted data, and have aligned true/false criteria.
- [ ] Intermediate `jq` deliberately retains only `file` and the typed local Noul needed by the aggregate stage. Final `jq` deletes top-level `.items`, so stdout contains aggregate/gate evidence but no source or paths.
- [ ] Gate reads the aggregate Noul, makes no API request, and preserves exact CLI exits: pass 0, reject 10, uncertain 11. `pipefail` must preserve nonzero gate status even though final `jq` consumes its JSON output.
- [ ] README explains the record shape after every stage, request count (`N` map calls + one reduce call; gate/jq offline), privacy/cost, why map and reduce solve different questions, safe-output behavior, and a NUL-safe Git changed-file invocation. State that this covers all decision pipeline primitives; `ask` remains the standalone one-state primitive.
- [ ] Fake-API behavioral tests start from two ordered fixture files and verify exact request sequence/count and state at both map calls and reduce call, intermediate shape, final source removal, response extras, pass/reject/uncertain output plus exits, and no request on invalid input. No model-quality assertions or grep/regex documentation tests.
- [ ] Run bounded live smoke using the two fixtures only, record model/version, aggregate answer, per-stage usage/request count, and gate result as observation—not deterministic truth or CI assertion.
- [ ] Focused/full checks, govulncheck, shell formatting/syntax, architecture/risk check, independent QA, report, and commits.

## Non-goals
- No change to existing code-smell example, CLI production behavior, universal review policy, parallel map execution, Git parser inside gev, automatic refactor, generated rationale, or attempt to pipe model text into execution.

## Notes
This example is intentionally more expensive than direct reduce: map spends one request per file and reduce spends one more. Its purpose is to teach explicit Unix composition, not recommend redundant model calls when one aggregate judgment is enough.

