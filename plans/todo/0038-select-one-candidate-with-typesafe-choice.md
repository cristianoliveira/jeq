---
id: TASK-0038
title: Select one candidate with TypeSafe Choice
status: doing
depends_on: []
priority: high
tags: [cli, typesafe, composability]
---

# Select one candidate with TypeSafe Choice

## Problem

JEQ exposes raw Choice requests, but callers must construct question JSON and
reconnect the selected id to their original candidate. This makes routing,
reranking, semantic selection, and pre-parsed extraction harder than necessary
and encourages separate domain-specific wrappers for the same operation.

## Desired outcome

A caller can pipe a bounded collection of candidate records into one general
`jeq pick` command. JEQ asks one TypeSafe Choice question, returns the selected
original record, and attaches the complete typed evidence. Callers remain in
control of candidate discovery, thresholds, shortlisting, and actions.

Example target workflow:

```sh
cat handlers.ndjson |
  jeq pick \
    --as route \
    --state-file request.txt \
    --instruction 'Which handler best fits this request?' \
    --id-pointer /name \
    --criteria-pointer /description
```

The selected record should remain directly usable by the next Unix process,
with the complete response available under `_jeq.route`.

## Acceptance criteria

### Candidate contract

- [ ] `jeq pick` accepts candidate objects as JSON array or NDJSON, using the
      established `--input json|ndjson` convention.
- [ ] `--id-pointer` resolves a unique, non-empty string id for every candidate.
- [ ] `--criteria-pointer` resolves the TypeSafe Choice description for every
      candidate and preserves supported structured JSON descriptions.
- [ ] The command rejects an empty or invalid candidate set, duplicate ids,
      missing pointer values, non-string ids, and more than TypeSafe's 255 Choice
      options before credential lookup or network access.
- [ ] JEQ does not invent a `none` option. A caller that needs “nothing fits”
      supplies it as an ordinary explicit candidate.

### Request and response

- [ ] `--instruction` becomes the single Choice question and `--as` is the
      stable evidence/question name.
- [ ] Exactly one explicit state source is required from `--state`,
      `--state-file`, or `--state-json`; candidate stdin cannot also be reused as
      state stdin.
- [ ] One successful invocation makes exactly one System One request regardless
      of candidate count and follows existing model, endpoint, timeout, retry,
      authentication, and interruption behavior.
- [ ] The selected id must match an input candidate. Otherwise JEQ returns a
      response-contract error and emits no misleading selected record.
- [ ] Success writes the selected original candidate as one JSON object with the
      complete lossless TypeSafe response under `_jeq.<as>`, following existing
      collision and deterministic-encoding rules.
- [ ] Choice, complete probability distribution, confidence, resolved model,
      usage, and unknown response fields remain available in the evidence.

### Composition and policy

- [ ] `pick` performs selection only. It does not hide probabilities, choose
      thresholds, filter to top-k, apply gates, trigger actions, or automatically
      perform a second-stage rerank.
- [ ] Its output pipes unchanged into `jq`, `jeq gate`, or another explicit JEQ
      stage. Documentation demonstrates at least one such composition.
- [ ] Help and examples explain that Choice ranks candidates relatively; a
      separate explicit candidate or Noul judgment is needed when all candidates
      may be unsuitable.

### Verification and documentation

- [ ] Domain and CLI tests are written first and cover happy paths plus every
      validation/failure path above with a fake evaluator or `httptest.Server`.
- [ ] A black-box test verifies stdin/stdout/stderr, exit status, and one-request
      behavior without production network access.
- [ ] Root help, `jeq pick --help`, README command tables, architecture guide,
      and a self-contained executable example describe the new primitive.
- [ ] `nix develop -c make check` passes.

## Constraints

- Keep `cmd/jeq` as the composition root and preserve current dependency arrows.
- Reuse existing source, contract, pipeline-envelope, renderer, and HTTP seams;
  do not create a second request path.
- Keep output lossless JSON and diagnostics plain text on stderr.
- Tests and normal CI stay offline and deterministic.
- Treat candidate content as untrusted data, never instructions for JEQ itself.

## Non-goals

- Candidate discovery, regex extraction, embeddings, or generative extraction.
- Automatic hierarchical selection for more than 255 candidates.
- Domain-specific `route`, `rank`, `skill`, or `extract` commands.
- Built-in policy thresholds or a default “none” candidate.
- Changes to `ask`, `map`, `reduce`, or `gate` semantics.
- Paid live verification in the normal test suite.

## Product evidence

- TypeSafe Choice returns one selected option, the complete probability
  distribution, and confidence.
- TypeSafe's pre-parsed extraction and skill-suggestion cookbooks both compose
  candidate discovery with Choice selection rather than generation.
- The current API permits at most 255 Choice options and recommends code-owned
  policy and multi-stage narrowing when a set is larger.
