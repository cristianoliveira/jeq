---
id: TASK-0009
title: TOON renderer with conformance spike
status: todo
depends_on: [TASK-0008]
priority: normal
tags: []
---

# TOON renderer with conformance spike

## Problem
TOON is the accepted default output but toon-go has no tagged release; adopting it blind risks corrupt output.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Time-box a dependency spike: record candidate, immutable revision/version, license, maintenance signal, and gaps against the current TOON specification; do not adopt an unverified encoder.
- [ ] Conformance corpus covers nested objects/arrays, empty values, Unicode, escaping, numeric precision, booleans/null, question-id keys, probabilities, legends, usage, and error documents.
- [ ] For every supported response/error fixture: value → TOON → decoded JSON is semantically identical; unsupported losslessness fails explicitly rather than degrading output.
- [ ] Output is deterministic, exactly one document plus one trailing newline, and uses the injected Renderer port.
- [ ] TOON becomes the default; `--output json` remains lossless JSON; invalid output names exit 2 with valid alternatives.
- [ ] Dependency is pinned in go.mod/go.sum and isolated behind infra/render so it can be replaced.

## Notes
No release until the semantic round-trip corpus passes. If no candidate passes, stop and document the blocker rather than writing an ad-hoc partial format.

