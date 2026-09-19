---
id: TASK-0030
title: Use plain text for human CLI interactions
status: done
depends_on: []
priority: high
tags: []
---

# Use plain text for human CLI interactions

## Problem
Human-facing jeq commands and standard failures are encoded as JSON even when they are brief prose/status information. This makes normal CLI use noisy. Keep structured JSON only for composable result data.

## Desired outcome
`jeq` behaves like a normal Unix CLI for people: help, discovery, status, validation receipts, and failures are concise plain text. JSON is reserved for lossless/composable evaluation results.

## Output boundary
- Result streams: `ask`, `map`, `reduce`, and `gate` success output stays JSON/NDJSON because downstream programs consume it.
- Human interactions: root home, `examples`, `version`, `models`, `validate`, help, and every error are plain text.
- Diagnostics and failures use stderr; successful human text and result data use stdout.

## Acceptance criteria
- [x] Remove the global output-format flag and format-selection abstraction. Do not offer JSON/TOON selection for home, examples, version, models, validate, or errors. Result commands emit their fixed JSON contract without requiring a format flag.
- [x] `jeq` prints a compact plain-text home with identity/purpose, credential readiness, resolved default model/source, available commands, and `Next: jeq examples`. No JSON punctuation or ANSI/TTY-dependent formatting.
- [x] `jeq examples` and `jeq examples <id>` render deterministic plain text. Preserve catalog/detail information, copyable multiline shell, request cost, input/output shape, privacy, exit semantics, and next step. Discovery remains offline and checkout-free.
- [x] `jeq version`, `jeq models`, and successful `jeq validate` render concise deterministic plain text. Model output preserves name, description, and release date. Unknown model response fields need not appear in human output.
- [x] `ask`, `map`, `reduce`, and `gate` successes remain lossless JSON/NDJSON with one trailing newline per document/record; pipeline behavior and gate exits 0/10/11 do not change.
- [x] Use Cobra's standard plain-text error/help rendering rather than a custom error formatter. All failures—including unknown commands/flags, local validation, auth, network, API, and per-record stream failures—write only to stderr, leave stdout empty before any result records, retain the domain code in error text where applicable, hide wrapped dependency details, and preserve existing exit classification.
- [x] Help remains Cobra plain text on stdout with exit 0 and no stderr. Use the conventional `SilenceUsage: true` pattern so operational failures do not dump unrelated usage; Cobra owns the `Error:` prefix, unknown-command suggestions, flag errors, and help wording.
- [x] Remove obsolete `--output` examples/tests/documentation and update ADR 0001 (plus supersede conflicting ADR 0003) to record the human-text/result-JSON boundary. Passing `--output` now follows normal unknown-flag behavior (exit 2, plain stderr).
- [x] Tests first: exact/golden human output; stdout/stderr routing; every exit class; result JSON validity/losslessness; map/gate streaming errors; offline discovery; no ANSI/TTY/locale variance; regression for secrets/raw dependency failures.
- [x] Run full checks, govulncheck, architecture/risk review, binary black-box verification, independent QA, and commit all work.

## Constraints
- Do not add a second general renderer, color, tables requiring terminal width, interactive prompts, TOON, TTY detection, or pretty JSON.
- Keep text stable and script-tolerable, but do not promise a machine schema for human commands/errors.
- Keep domain errors structured internally; translate only at the CLI boundary.

## Notes
This intentionally revises the earlier agent-first all-JSON contract. Plain text is the correct representation for brief human interaction; structured result data remains composable.

