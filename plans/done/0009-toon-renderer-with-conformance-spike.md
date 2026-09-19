---
id: TASK-0009
title: TOON conformance spike and safe fallback
status: done
depends_on: [TASK-0008]
priority: normal
tags: []
---

# TOON conformance spike and safe fallback

## Problem
TOON was the proposed default, but no Go implementation may be shipped unless it preserves arbitrary server JSON and conforms to the current specification.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Test candidate encoders at immutable revisions against current official TOON v4.1.1 fixtures and the complete gev/adversarial arbitrary-JSON corpus.
- [ ] Record exact pass/fail counts, licenses, maintenance state, embedded spec revisions, and representative gev-relevant failures.
- [ ] Reject any candidate that emits noncanonical current-spec output for values preserved from unknown server fields; semantic self-round-trip alone is insufficient.
- [ ] With no conformant candidate, ship v1 JSON-only: remove TOON runtime/dependency, keep deterministic single-document JSON, and reject non-JSON output names.
- [ ] ADR 0003 records the evidence, supersedes ADR 0001's TOON-default clause, and states an objective revisit condition.
- [ ] Full gate passes with no TOON module in the dependency graph.

## Notes
Spike result: official `toon-format/toon-go` passed 414/538 v4.1.1 fixtures; `jedi-knights/go-toon` passed 514/538 but fails nested field-group canonical encoding reachable through arbitrary unknown fields. Do not ship either.

