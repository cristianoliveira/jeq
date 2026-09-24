---
id: TASK-0072
title: Standardize inline JSON and file input flags
status: done
depends_on: []
priority: high
tags: [cli, ux, inputs, compatibility]
---

# Standardize inline JSON and file input flags

## Problem
`--questions-json` means inline JSON on map/reduce, but `--state-json` means a file path on ask/rank/validate. Passing inline state JSON produces a file-not-found error. ask/validate also lack the inline question option available on map/reduce. Users cannot transfer what they learned between commands.

## Desired outcome
A flag keeps the same source meaning wherever it exists. Users can correctly predict inline versus file input without jeq guessing from argument contents.

## Proposed source contract
- `--state`: literal text.
- `--state-file`: text file, or selected stdin via `-` where permitted.
- `--state-json`: inline JSON.
- `--state-json-file`: JSON file, or selected stdin via `-` where permitted.
- `--questions`: question-document file, retaining its established file-only meaning.
- `--questions-json`: inline question-document JSON for ask, validate, map, and reduce.
- `--request`: complete native request file, retaining its established file-only meaning.

## Acceptance criteria
- [x] Apply the source contract consistently to relevant commands. Do not add question-document flags to rate/rank, which construct their own question shapes.
- [x] Reject conflicting sources and multiple stdin consumers before reading input, loading credentials, or creating a client.
- [x] Preserve explicit native/composed modes and object/array/NDJSON framing. No content sniffing, automatic framing detection, or implicit provider calls.
- [x] Inline JSON receives the same strict decoding, duplicate-key checks, and size limits as equivalent file input.
- [x] Malformed inline JSON produces an input error, not a file lookup. A filename passed to the new `--state-json` meaning receives concrete migration guidance without opening it.
- [x] Flag values and input payloads are not echoed into new diagnostics; examples use synthetic data and explain when a file/stdin avoids shell-history exposure.
- [x] Update executable examples, help, guides, and affected scripts together. Publish the breaking migration from `--state-json FILE` to `--state-json-file FILE`; do not silently reinterpret old invocations.
- [x] Table-driven tests compare the outgoing request for equivalent inline/file/stdin inputs and cover malformed input, conflicts, bounds, and no-network failures.
- [x] Focused tests and the configured watcher gate pass.

## Constraints
jeq remains stateless. Input selection applies only to the current invocation; no remembered input path, last request, alias learning, or config writes. Keep established file options explicit rather than adding heuristic compatibility.

## Context
Inspect `internal/cli/questions.go`, `ask.go`, `validate.go`, `rank.go`, and domain source/composition rules. This is an intentional v0.x interface change, not a documentation-only patch. Coordinate shared help/recipe updates with TASK-0071 and TASK-0075.

## Completion
- Code commits: `6eb75eb`, `ebfb1b8`.
- Verification: `go test ./internal/cli ./internal/domain/contract ./internal/domain/jeq ./internal/blackbox` passed; configured watcher generation 59 passed.
- Migration: previous `--state-json FILE` usage now requires `--state-json-file FILE`.
