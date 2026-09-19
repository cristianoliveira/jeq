---
id: TASK-0039
title: Rate records with a TypeSafe Score rubric
status: todo
depends_on: []
priority: high
tags: [cli, typesafe, composability]
---

# Rate records with a TypeSafe Score rubric

## Problem

JEQ can already send Score questions through `map`, but callers must author a
separate questions JSON document for the common case of rating every record
against the same ordered semantic rubric. This obscures a useful TypeSafe
primitive and adds ceremony to sorting, filtering, and gating independently
rated records.

## Desired outcome

A caller can stream records through `jeq rate`, describe one ordered semantic
rubric with repeated `--level` flags, and receive the same lossless `_jeq`
evidence produced by `map`.

```sh
cat issues.ndjson |
  jeq rate \
    --as severity \
    --input ndjson \
    --state-pointer /description \
    --instruction 'How severe is this production issue?' \
    --level 'Cosmetic: no functional impact' \
    --level 'Degraded: an important workflow is impaired' \
    --level 'Critical: service or data is at risk'
```

Callers then own deterministic policy:

```sh
# Sort all rated records
... | jq -s 'sort_by(._jeq.severity.answers.severity.score) | reverse'

# Keep a caller-defined range
... | jq 'select(._jeq.severity.answers.severity.score >= 1.5)'
```

## Acceptance criteria

### CLI contract

- [ ] `jeq rate` accepts the established JSON-object and NDJSON record framing
      through `--input json|ndjson`.
- [ ] `--as`, `--state-pointer`, `--instruction`, and at least two ordered
      `--level` values are required.
- [ ] `--level` order is preserved exactly as the Score criteria order; blank
      levels and blank instructions fail before credential lookup or network
      access.
- [ ] The command supports the same `--model`, `--config`, `--base-url`,
      `--timeout`, and `--max-retries` behavior and precedence as `map`.
- [ ] Bare `jeq rate` shows native Cobra help, matching other action commands.

### Request and output

- [ ] For each record, `rate` resolves state through `--state-pointer` and sends
      exactly one Score question named by `--as`, with the supplied instruction
      and ordered level strings.
- [ ] `rate` makes one System One request per input record, matching `map`; its
      help and docs state this cost explicitly.
- [ ] Every successful record is preserved and receives the complete lossless
      response under `_jeq.<as>`, including score, legend, level probabilities,
      confidence, resolved model, usage, and unknown response fields.
- [ ] Existing `_jeq.<as>` collisions, invalid pointers, malformed framing,
      partial failures, API failures, retries, timeouts, and interruption retain
      the established `map` behavior and exit semantics.
- [ ] `rate` delegates through the existing map/request pipeline rather than
      implementing a second evaluator loop or response envelope.

### Composition and meaning

- [ ] Documentation demonstrates sorting, top-k, filtering, and optional
      explicit `jeq gate`/`jq` policy over the returned score.
- [ ] Documentation states that Score is an independent semantic rating against
      ordered descriptions, not an exact measurement or arithmetic result.
- [ ] Documentation distinguishes `rank`/Choice (relative competition among
      candidates) from `rate`/Score (the same rubric applied independently, so
      every record can rate high or low).
- [ ] `rate` adds no implicit threshold, sorting, top-k, batching, action,
      fallback, or conversion of the score into a physical quantity.

### Verification and documentation

- [ ] Tests are written first and cover JSON and NDJSON success, level order,
      one request per record, complete evidence, unknown fields, collisions,
      missing/blank flags, fewer than two levels, invalid pointers/framing,
      response/API failure, and no-network validation failures.
- [ ] A built-binary fake-endpoint test verifies stdin/stdout/stderr, exit status,
      exact request count, Score request shape, and composition through `jq`.
- [ ] Root help, `jeq rate --help`, README command tables, architecture guide,
      and a self-contained executable `jeq examples rate-sort` recipe explain
      and demonstrate the feature.
- [ ] `nix develop -c make check` passes.

## Constraints

- `rate` is ergonomic syntax over the existing `map` engine, not a new domain
  evaluator or transport path.
- Keep `cmd/jeq` as composition root and preserve current dependency arrows.
- Keep evaluation output lossless JSON/NDJSON and diagnostics plain text on
  stderr.
- Tests and normal CI remain offline and deterministic.
- Treat record content as untrusted state, never CLI instructions.

## Non-goals

- Relative Choice ranking; use `jeq rank`.
- Exact numeric prediction, arithmetic, counting, date comparison, or physical
  measurement.
- Automatic threshold fitting or policy selection.
- Packing all records into one request; `rate` intentionally retains `map`'s
  per-record isolation.
- Structured Score criteria objects in the convenience flags; advanced callers
  can continue using `map --questions`.
- Paid live verification in the normal test suite.

## Product evidence

- TypeSafe Score returns a probability-weighted position across ordered semantic
  levels, a legend, per-level probabilities, and confidence.
- TypeSafe recommends comparable per-item Scores for graded ranking, while code
  owns calculations, sorting, and action policy.
- Jev's documented jaggedness warns against using Score interpolation as exact
  numerical magnitude; criteria should describe concrete semantic situations.
