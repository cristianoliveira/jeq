---
id: TASK-0023
title: Versioned decision workflow engine
status: doing
depends_on: [TASK-0022]
priority: high
tags: []
---

# Versioned decision workflow engine

## Problem
Complex gev decisions currently require a Python subprocess adapter. The domain needs a bounded, declarative, side-effect-free workflow model that can express proven routing, gating, deterministic prechecks, and justified cascades without embedding a general programming language.

## Desired outcome
A small domain engine expresses the semantics already proven by support routing, release readiness, and incident triage while remaining inspectable, bounded, and incapable of executing actions.

## Acceptance criteria
- [ ] ADR 0004 defines workflow v1, threat boundaries, why JSON is used, and explicit exclusions; the manifest has `version`, `name`, `input`, bounded `preconditions`, bounded sequential `stages`, ordered terminal `outcomes`, and a required default outcome.
- [ ] Strict decoding rejects duplicates, unknown fields, unsupported versions/operators, duplicate IDs, invalid exits, empty conditions, unreachable/ambiguous references, and manifests above documented stage/condition/candidate limits.
- [ ] Conditions support only `all`/`any`/`not` and typed leaf comparisons `eq`, `gte`, `lte`, `non_empty` over allowlisted input JSON pointers or prior answer fields; no script, interpolation, regex, template, environment expansion, or arbitrary expression evaluation exists.
- [ ] Stages use either a local questions document or one generated Choice whose criteria come from a local category-keyed catalog selected by a prior allowlisted Choice answer.
- [ ] Runtime resolves ordered preconditions before network, evaluates sequential stages, stops at the first matching terminal outcome, validates every model-derived lookup, caps four stages/requests, and aggregates usage.
- [ ] Generic receipt contains workflow/version, decision/reason, matched outcome, stage count, per-stage typed answer/model/usage evidence, and aggregate usage; raw input and credentials are never emitted.
- [ ] Engine is dependency-injected behind an evaluation port, deterministic under fixtures, and independent of Cobra/filesystem/HTTP/rendering.
- [ ] Tests cover zero-call blockers, one-stage route/gate, two-stage catalog selection, fallthrough/default, malformed answer/lookup, limits, usage aggregation, and interruption/operational propagation.

## Security constraints
- Treat manifests, state, referenced files, and model answers as untrusted.
- Workflow paths must later resolve beneath the manifest directory; no URLs, symlink escape, commands, side effects, or model-selected path.
- Policy exits are only `0`, `10`, or `11`; gev operational/usage/interruption remain `1`, `2`, and `130`.

## Notes
The Python examples are executable specifications, not the target runtime. Keep the engine small enough to explain without a second programming language.

