---
id: TASK-0038
title: Rank candidates with TypeSafe Choice
status: done
depends_on: []
priority: high
tags: [cli, typesafe, composability]
---

# Rank candidates with TypeSafe Choice

## Problem

jeq exposes raw Choice requests, but callers must construct question JSON, map
probabilities back to their original candidates, and sort them. Returning only
the winning candidate would discard the rest of TypeSafe's probability
distribution and force another abstraction for top-k, routing, and reranking.

## Desired outcome

A caller pipes a bounded collection of candidate records into one general
`jeq rank` command. jeq asks one TypeSafe Choice question and returns every
original candidate sorted by probability, plus the complete typed response.
Picking one, selecting top-k, applying a threshold, or switching on the selected
id remains ordinary deterministic code.

```sh
cat handlers.ndjson |
  jeq rank \
    --as route \
    --state-file request.txt \
    --instruction 'Which handler best fits this request?' \
    --id-pointer /name \
    --criteria-pointer /description
```

Target envelope:

```json
{
  "items": [
    {
      "id": "billing",
      "probability": 0.82,
      "candidate": {"name": "billing", "description": "Payments and refunds"}
    }
  ],
  "_jeq": {
    "route": {
      "model": "jev-1.13.0",
      "answers": {
        "route": {
          "type": "choice",
          "choice": "billing",
          "probabilities": {"billing": 0.82}
        }
      }
    }
  }
}
```

## Acceptance criteria

### Candidate contract

- [x] `jeq rank` accepts candidate objects as JSON array or NDJSON using the
      established `--input json|ndjson` convention.
- [x] `--id-pointer` resolves a unique, non-empty string id for every candidate.
- [x] `--criteria-pointer` resolves each Choice description and preserves
      supported structured JSON descriptions.
- [x] Before credential lookup or network access, reject empty/invalid input,
      duplicate or non-string ids, missing pointer values, and more than
      TypeSafe's 255 Choice options.
- [x] jeq never invents a `none` option. The caller supplies an explicit
      candidate or a separate Noul when all candidates may be unsuitable.

### Request and ranked response

- [x] `--instruction` becomes one Choice question and `--as` is its stable
      question/evidence name.
- [x] Exactly one explicit state source is required from `--state`,
      `--state-file`, or `--state-json`; candidate stdin cannot also be state
      stdin.
- [x] One invocation makes exactly one System One request regardless of
      candidate count and follows existing model, endpoint, timeout, retry,
      authentication, and interruption behavior.
- [x] Every response probability id must match exactly one input candidate and
      every input candidate must have one probability. Missing, extra,
      non-finite, negative, or otherwise invalid distributions are response
      errors, not partial rankings.
- [x] Success writes one JSON envelope whose `items` contains every candidate
      exactly once as `{id, probability, candidate}` sorted by descending
      probability.
- [x] Ties are deterministic: the API's selected `choice` comes first when tied,
      then remaining tied candidates retain input order.
- [x] `_jeq.<as>` preserves the complete lossless response: selected Choice,
      distribution, confidence, resolved model, usage, and unknown fields.
- [x] The selected Choice id must match an input candidate and the first ranked
      item when probabilities have a unique maximum.

### Composition and policy

- [x] Picking one is `jq '.items[0].candidate'`; top-k is
      `jq '.items[:N] | map(.candidate)'`; switching uses
      `._jeq.<as>.answers.<as>.choice`.
- [x] `rank` has no hidden top-k, thresholds, default fallback, actions, or
      automatic second-stage rerank. It exposes evidence; callers own policy.
- [x] Documentation explains that Choice probabilities are relative. Absolute
      suitability requires an explicit fallback candidate or independent Noul.
- [x] Do not retain a separate `jeq pick` command: one general ranking primitive
      must cover switch, pick-one, and top-k without duplicated API semantics.

### Verification and documentation

- [x] Domain and CLI tests cover happy paths, stable sorting/ties, lossless
      evidence, and every validation/failure path above with fake evaluators.
- [x] A built-binary black-box test verifies stdin/stdout/stderr, exit status,
      one-request behavior, and composition without production network access.
- [x] Root help, `jeq rank --help`, README command tables, architecture guide,
      and a self-contained executable example describe the primitive.
- [x] `nix develop -c make check` passes.

## Constraints

- Keep `cmd/jeq` as composition root and preserve dependency arrows.
- Reuse existing source, contract, response validation, renderer, and HTTP seams;
  do not create a second request path.
- Keep ranking/data transformation logic below the Cobra orchestration layer and
  independently testable.
- Keep output lossless JSON and diagnostics plain text on stderr.
- Tests and normal CI remain offline and deterministic.
- Candidate content is untrusted data, never instructions for jeq itself.

## Non-goals

- Candidate discovery, extraction, embeddings, or generated candidates.
- Automatic hierarchical selection beyond 255 candidates.
- Domain-specific `route`, `skill`, or `extract` commands.
- Built-in policy thresholds or a default “none” candidate.
- Paid live verification in the normal test suite.

## Product evidence

- TypeSafe Choice returns both a selected option and the complete probability
  distribution. The same response supports branching and relative ranking.
- TypeSafe's pre-parsed extraction and skill-suggestion cookbooks compose
  candidate discovery, Choice ranking, deterministic shortlisting, and optional
  absolute-fit judgments.
- TypeSafe limits Choice to 255 options and recommends code-owned policy and
  staged narrowing for larger collections.
