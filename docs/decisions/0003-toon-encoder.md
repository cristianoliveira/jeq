# 0003. TOON encoder adoption

- Status: Accepted
- Date: 2026-09-19

## Decision

Adopt `github.com/toon-format/toon-go` at immutable revision
`7ca0e27c4e8c695d99c88ca4a123f409086da91e`, resolved by Go as
`v0.0.0-20251202084852-7ca0e27c4e8c`. The dependency is MIT licensed and is
isolated to `internal/infra/render` behind the existing `cli.Renderer` port.

The candidate's spec-fixture submodule is pinned at
`51fe1e901c6dca6ca51137e20a3345ef8bcae47e`, released as TOON specification
v1.4.0 (`tests/spec`). The candidate repository has no tagged Go release; the
pseudo-version and commit are therefore part of this decision's compatibility
surface.

## Conformance evidence

The upstream encoder/decoder fixture suite passes at the pinned revision.
Gev's corpus covers the full response fixture (answers, probabilities,
confidence, score legends, usage, and unknown fields), error documents, nested
objects/arrays, empty values, Unicode, escaping, numeric values, booleans,
null, and question-id keys. Each document is checked as value → TOON → decoded
value; deterministic output and exactly one trailing newline are asserted.

TOON cannot represent an unsafe IEEE-754 integer without changing its value.
Gev therefore verifies every emitted document's semantic round trip and fails
before writing bytes when the candidate would lose numeric precision. This is
a deliberate hard failure, not a lossy fallback.

The default renderer is TOON. JSON remains available through `--output json`.
Both formats are deterministic and use the same renderer port.
