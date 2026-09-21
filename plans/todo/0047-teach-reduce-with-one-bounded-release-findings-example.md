---
id: TASK-0047
title: Teach reduce with one bounded release-findings example
status: todo
depends_on: []
priority: high
tags: [documentation, examples, reduce, onboarding]
---

# Teach reduce with one bounded release-findings example

## Problem
Users can see that reduce judges a collection once, but they lack one small executable example that shows input shape, aggregate evidence, cost, policy direction, and how reduce differs from map, rate, rank, and gate.

## Desired outcome
A reader can answer “when and how do I use reduce?” from one short guide, then run one bounded release-findings example without inferring request shape or policy direction.

## Context
`reduce` sends the complete bounded collection in one TypeSafe request and appends one aggregate judgment under `_jeq.<name>`. It is distinct from per-record `map`/`rate`, relative `rank`, and offline `gate`. Use a positively phrased `release_ready` Noul question so a high value means pass and a low value means reject; avoid the inverted “should this block?” trap.

## Acceptance criteria
- [ ] Add a focused `docs/guides/reduce.md` explaining when to use reduce, when not to use it, one-request cost, collection size/privacy limits, JSON/NDJSON input, evidence path, and comparison with map/rate/rank/gate.
- [ ] Add one small executable example under `examples/` with synthetic release findings, a reusable questions document, and a strict shell entry point that accepts `JEQ_BIN` and preserves stdout/stderr/exit status.
- [ ] The example runs `reduce` exactly once over the complete bounded collection and names aggregate evidence `release_ready`.
- [ ] The guide shows the expected `_jeq.release_ready.answers.release_ready.noul` shape and an optional offline `gate` using the exact RFC 6901 pointer.
- [ ] Gate direction is intuitive: high release-readiness probability passes, low probability rejects, middle is uncertain.
- [ ] README and examples index link the reduce guide/example without expanding the landing page substantially.
- [ ] Automated behavior tests prove the entry point passes exact input/arguments to an injected fake JEQ and propagates failures; tests do not merely grep documentation.
- [ ] Offline tests and the normal gate pass; no live TypeSafe request is required.

## Constraints and non-goals
- Keep this a singular teaching example, not a multi-stage release framework.
- Do not execute deployment or release actions.
- Do not claim probability thresholds are calibrated defaults; label them illustrative.
- Do not add a hidden collection-size policy to JEQ.
- Keep fixtures synthetic and free of credentials or proprietary data.

