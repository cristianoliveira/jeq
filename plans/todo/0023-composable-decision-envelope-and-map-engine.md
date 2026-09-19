---
id: TASK-0023
title: Composable decision envelope and map engine
status: doing
depends_on: [TASK-0022]
priority: high
tags: []
---

# Composable decision envelope and map engine

## Problem
Single-response commands force callers to discard or manually rejoin their input. A monolithic workflow would hide intermediate stages. Unix composition needs a stable record that survives each semantic transformation.

## Desired outcome
One gev operation can enrich one JSON record with one named typed judgment, and its output can feed the same operation again without losing input data or previous evidence.

## Acceptance criteria
- [ ] ADR 0004 defines the composability law, envelope v1, collision/privacy policy, and why workflows are deferred until they can compile to visible primitives.
- [ ] An envelope is a strict JSON object; arbitrary original members remain byte-semantically lossless, while gev appends one result at reserved `_gev.<name>`.
- [ ] Names are bounded safe identifiers; existing `_gev.<name>` is rejected rather than overwritten; pre-existing `_gev` must be an object with strict duplicate-key handling.
- [ ] RFC 6901 JSON Pointer selects the state value from the original record. Missing/invalid pointers and unsupported root shapes fail locally before evaluator construction.
- [ ] One injected evaluator receives a normal composed TypeSafe request and returns a typed response; the engine appends answers/model/usage without interpreting them or executing actions.
- [ ] Transformation is deterministic, preserves unknown input/server fields and numeric precision, emits no credentials, and never uses model output as a path, command, or authority.
- [ ] Domain code is independent of Cobra, files, stdin, HTTP, and rendering; operational/interruption errors propagate unchanged.
- [ ] Tests cover root/nested/escaped pointers, arbitrary values, Unicode, unsafe numbers, existing evidence, collisions, duplicate keys, missing state, typed response evidence, and evaluator failures.

## Security constraints
- Treat records, pointers, question documents, and model answers as untrusted.
- No expression language, branching, actions, templates, path lookup, environment expansion, or arbitrary execution.
- Enrichment intentionally carries input forward; documentation must warn callers not to pipe secrets into logs and later CLI must offer explicit projection through standard tools.

## Notes
This is one-record transformation only. TASK-0024 owns JSON/NDJSON iteration, limits, policy gates, and removal of temporary Python wrappers.

