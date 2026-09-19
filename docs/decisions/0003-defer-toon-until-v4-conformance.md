# 0003. Defer TOON until v4 conformance

- Status: Superseded by 0030
- Date: 2026-09-19

## Context

Version 1 must emit deterministic, lossless machine output. TOON was evaluated
as a compact alternative, but the first candidate targets an older specification
and the second candidate is not canonical across the complete JSON value domain
that `gev` preserves through unknown fields and raw JSON members.

The current reference is TOON specification v4.1.1:
`62f16b369408180f1faf1cba7da1b46d1f336f12`.

## Evidence

The detailed temporary investigation is recorded in
`.tmp/reports/19-09-26/toon-v4-conformance.md` and is intentionally not part of
the commit. The official v4.1.1 fixture corpus contains 538 cases:

- `github.com/toon-format/toon-go` at
  `7ca0e27c4e8c695d99c88ca4a123f409086da91e`: **414/538 pass**;
- `github.com/jedi-knights/go-toon` at
  `e7c64359c4404d1c3642f625a450ffef0f268f09`: **514/538 pass**.

The second candidate passes the existing GEV response, error, home, version,
models, and validate semantic corpus (19/19) and common adversarial values.
However, it fails seven canonical encoder fixtures involving nested field groups.
For example, a JSON array of uniform objects with a nested uniform object must
emit a fields-bearing tabular header in v4.1.1, not list form. Unknown response
fields and other preserved raw JSON values can contain this shape, so the gap
cannot be excluded from GEV's supported value domain.

## Decision

Version 1 result streams remain **JSON-only**:

- `ask`, `map`, `reduce`, and `gate` emit JSON/NDJSON result data;
- human discovery, validation, models, version, and errors use deterministic plain text;
- there is no global `--output` selector;
- no TOON runtime code or dependency is shipped.

This is a forward correction to the earlier TOON adoption. History is not
rewritten.

## Revisit condition

Reconsider TOON only when a candidate passes the complete current v4.1.1
**encoder** corpus and the complete GEV semantic corpus, including arbitrary
preserved JSON values: unknown fields and keys, nested objects and arrays,
heterogeneous arrays, empty values, Unicode/control strings, numeric-like
values, and unsafe-number handling. Decoder completeness alone is insufficient.
