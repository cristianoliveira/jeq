# JEQ QA acceptance plan — D0–D2 (TASK-0016)

Status: accepted for D1 verification — open questions OQ-1..OQ-7 resolved in commit 92926aa and folded into the checks below.
Ground truth: ADR 0001 (agent-first CLI contract), ADR 0002 (layered architecture), plans/todo TASK-0001..0011.
Exit-class truth from ADR 0001: `0` success · `1` auth/API/network/timeout/response failure · `2` usage or locally invalid input · `130` interrupted. Low confidence is never an error.

**Error motto (product rule): every failure is auto-discoverable — non-zero is never silent.** Every non-zero exit emits **exactly one structured error document on stdout** in the selected format (ADR 0001: success and errors use the selected structured format on stdout), carrying the stable `JEQ_*` code, one actionable recovery instruction, and — for usage failures — the offending input plus the valid alternatives so the caller can self-correct without leaving the terminal. stderr remains diagnostics/retry-progress only: no raw Cobra prose, no duplicate error documents across channels.

**Phase split (PO decision, final):** the D0 bootstrap **permits a temporary stderr prose fallback** (commits ff1db92 + 650f0a2: usage failures print `Error: …` + help hint on stderr instead of exiting silently). TASK-0017/D2 **must replace it** with exactly one structured stdout error document and **remove** the raw Cobra prose/duplicate stderr. D2-17 is the final ADR contract; the fallback is not a pass for it.

## How to read this plan

- Every check is **Given / When / Then** plus the **exact local command** and/or **fixture** that verifies it.
- Fixtures live in `internal/fixtures/` (ADR 0002): `contract/` for request/response documents, `http/` for status-and-body scenarios, `requests/` for ready-to-send ask inputs.
- All commands run from the repo root. `make check` is the normal gate; targeted `go test -run` lines isolate a check.
- "Pre-network" is verified by a fake server counting requests (must be `0`) plus injected readers/env asserting no credential lookup or file access occurred.

## D0 — Foundation and error codes (TASK-0001, TASK-0002)

**D0-1 Normal gate is green and enforcing.**
Given a clean checkout, when `make check` runs, then it exits 0 and includes gofmt, vet, golangci-lint (with depguard/import-architecture rules per ADR 0002), race tests, and coverage.
Verify: `make check`.

**D0-2 Exit-class mapping is total and deterministic.**
Given every symbolic error code in the registry, when mapped to exit classes, then each maps to exactly one of `0/1/2/130` and the mapping is a pure function (no environment or I/O).
Verify: `go test ./internal/cli/... -run TestExitClassMapping` — table-driven, domain-owned table.

**D0-3 Symbolic codes are a stable registry.**
Given the code registry, when a code is added/removed/renamed, then a golden snapshot test fails until the change is explicit. Format locked to `JEQ_<AREA>_<REASON>`; initial registry per TASK-0002 (JEQ_AUTH_MISSING, JEQ_AUTH_REJECTED, JEQ_REQUEST_INVALID, JEQ_SOURCE_CONFLICT, JEQ_INPUT_INVALID, JEQ_RATE_LIMITED, JEQ_SERVER_ERROR, JEQ_RESPONSE_INVALID, JEQ_NETWORK_ERROR, JEQ_TIMEOUT, JEQ_INTERRUPTED).
Verify: `go test ./internal/domain/jeq/... -run TestErrorCodeRegistryGolden` — fixture `contract/error_codes.golden`.

**D0-4 Error documents follow the output contract.**
Given any failure, when the error document is emitted, then it uses the selected format on stdout, is exactly one document plus one trailing newline, and carries a stable code plus one actionable recovery instruction.
Verify: `go test ./internal/cli/... -run TestErrorDocumentShape` — fixture `contract/error_shape.golden`.

**D0-5 Errors never leak secrets or internals.**
Given a failure at any layer, when output is rendered, then stdout and stderr contain no API key, no stack traces, no raw dependency error text beyond the declared diagnostics channel.
Verify: `go test ./internal/cli/... -run TestErrorRedaction` — fixture `http/500.json` served with the key present in env.

**D0-6 Interruption maps to 130.**
Given a canceled/interrupted run, when the exit class is computed, then it is `130`, distinct from `1`.
Verify: `go test ./internal/cli/... -run TestInterruptClass`.

**D0-7 Confidence never changes exit class.**
Given any response with low probabilities or confidence, when classified, then the class is `0` (negative check: no confidence-based entry exists in the mapping table).
Verify: `go test ./internal/cli/... -run TestExitClassMapping -run 'Confidence'` — fixture `contract/response_low_confidence.json`.

**D0-8 Usage failures are never silent — bootstrap criterion (added post-verification, from finding F-1 in d0-verification.md).**
Given an unknown command or unknown flag, when the binary runs, then exit is `2` AND the failure is discoverable. **D0 bootstrap pass condition (PO-approved, met by ff1db92 + 650f0a2):** non-empty stderr naming the offending input with a help/alternatives pointer; `version` unaffected; test-asserted (`TestUsageFailuresAreNotSilent`). Cobra's did-you-mean suggestion passthrough is preserved.
**Final criterion (not met at D0, owned by TASK-0017, depends TASK-0008, blocks TASK-0011):** exactly one structured stdout error document — `JEQ_INPUT_INVALID`, offending input, valid commands/flags as recovery, one trailing newline — with raw Cobra prose and duplicate stderr removed; help stays exit 0 on stdout.
Verify (bootstrap): build `./cmd/jeq`; probes `./jeq badsubcommand`, `./jeq --nope` exit 2 with discoverable stderr; `go test ./internal/cli/... -run TestUsageFailuresAreNotSilent`.
Verify (final, via TASK-0017/D2-17): probes capture stdout = exactly one structured document (`jq -e .` parses, code present), stderr free of error prose; goldens per TASK-0017 (unknown command, unknown flag, misspelled-command suggestion, help).

## D1 — Contract types, validation, ask composition (TASK-0003..0005)

All D1 checks assert **exit `2` with a stable code, empty machine output, zero network requests, zero credential lookup**. Shared verify pattern: `go test ./internal/domain/... -run Test<Input>` with the named fixture; e2e variants via `go test ./internal/cli/... -run TestAskLocal`.

**D1-1 Malformed JSON fails locally.**
Given syntactically invalid JSON (truncated, trailing comma, BOM), when decoded, then exit `2`, code set, no network.
Fixture: `contract/malformed_truncated.json`, `contract/malformed_trailing_comma.json`.

**D1-2 Duplicate keys are rejected, not last-wins.**
Given a document with a duplicate object key, when strictly decoded, then exit `2` with a duplicate-key code.
Fixture: `contract/dup_key.json` (`{"state":"a","state":"b"}` shape analog in questions).

**D1-3 Unknown fields pass through in both modes (OQ-1 resolved).**
Given `--questions` or `--state-json` JSON containing an unrecognized field, when decoded and re-encoded, then the unknown field survives byte-semantically — never rejected, in composed and native mode alike. Strictness covers only duplicate keys and types of *known* fields.
Fixture: `contract/unknown_field.json`; verify: `go test ./internal/domain/contract/... -run TestUnknownFieldPassthrough`.

**D1-4 Type mismatches fail strict decode.**
Given a probability supplied as a string or levels as a scalar, when decoded, then exit `2`.
Fixture: `contract/type_mismatch.json`.

**D1-5 Response decode is lossless.**
Given a response containing fields jeq does not model, when decoded and re-rendered, then the unknown fields are preserved verbatim.
Fixture: `contract/response_unknown_fields.json`; verify: `go test ./internal/domain/contract/... -run TestLosslessResponse`.

**D1-6 Unknown primitive fails locally.**
Given a question whose primitive is not `noul|choice|score`, when validated, then exit `2` pre-network.
Fixture: `contract/unknown_primitive.json`.

**D1-7 Local rules are exactly the client-owned invariants (OQ-2 resolved).**
Given valid UTF-8 JSON, no duplicate keys, non-empty state, ≥1 question, known `type`, non-empty `instructions`, choice `criteria` non-empty map, score `criteria` ≥ 2 levels, noul `criteria` (if present) an object — each violated invariant exits `2` locally; everything semantic defers to server 422 (exit `1`).
Fixtures: `contract/choice_no_options.json`, `contract/score_no_levels.json`, `contract/missing_instructions.json`, `contract/noul_criteria_wrong_shape.json`.

**D1-8 Empty required sections fail locally.**
Given empty `questions`, or composed mode resolving to an empty state, then exit `2`.
Fixtures: `contract/empty_questions.json`, `contract/empty_state.json`.

**D1-9 Mode conflicts fail before any I/O.**
Given `--request` combined with `--questions`, or `--request` with any `--state*`, or two `--state*` flags together, when parsed, then exit `2` with the conflict code, and the fake server counted `0` requests and injected readers observed no file/env access.
Verify: `go test ./internal/domain/jeq/... -run TestModeConflictMatrix` (table = the full conflict matrix from ADR 0001).

**D1-10 Exactly one state source is accepted.**
Given each of `--state`, `--state-file`, `--state-json`, `--state-json -` alone with `--questions`, when composed, then composition succeeds and proceeds to evaluation (no conflict error).
Fixture: `requests/composed_questions.json` + per-source state fixtures.

**D1-11 stdin is never read implicitly.**
Given ask invoked with explicit non-stdin inputs while stdin is closed (not `-`), when the command runs, then it completes without blocking.
Verify: `go test ./internal/cli/... -run TestNoImplicitStdin` with injected closed stream; binary smoke: `jeq ask --questions q.json --state "x" </dev/null`.

**D1-12 Model resolution follows the documented chain.**
Given none/each of `--model`, `TYPESAFE_DEFAULT_MODEL`, set, when composing, then the resolved model is respectively `jev-latest` → env → flag (flag wins). Resolution lives in `internal/cli` (os.Getenv containment per ADR 0002); the domain receives the resolved model explicitly.
Verify: `go test ./internal/cli/... -run TestModelPrecedence` with `t.Setenv`.

**D1-13 Base URL override is honored.**
Given `TYPESAFE_BASE_URL` or `--base-url`, when the client is built, then requests target that root (flag precedence).
Verify: `go test ./internal/infra/typesafeapi/... -run TestBaseURL` (httptest server receives the call).

**D1-14 Every failure fixture maps to a stable code.**
Given the fixture directory, when the meta-test walks it, then each failure fixture declares a stable code and that code exists in the D0 registry.
Verify: `go test ./internal/fixtures/... -run TestFixtureCodeCoverage`.

## D2 — HTTP client, retries, JSON renderer, source readers, ask e2e (TASK-0006..0008, 0010, 0011)

Client checks run against `httptest.Server` (ADR 0002: real client, no transport mocks), with an injected clock/sleeper recording sleeps.

**D2-1 Auth is bearer-from-env only.**
Given `TYPESAFE_API_KEY` set, when a request is sent, then `Authorization: Bearer <key>` is present, and no key flag exists anywhere in the command tree.
Verify: `go test ./internal/infra/typesafeapi/... -run TestBearerAuth`; `go test ./internal/cli/... -run TestNoKeyFlag`.

**D2-2 200 success renders losslessly.**
Given a recorded 200 response, when ask completes, then exit `0` and stdout is exactly the golden JSON (resolved model, every answer, probabilities, confidence, score legends, usage) plus one trailing newline. JSON is the version 1 default and only supported output (TASK-0009 conformance spike deferred TOON).
Fixture: `contract/response_200_full.json` → golden `render/json_200_full.golden`; verify: `go test ./internal/infra/render/... -run TestJSONGolden`.

**D2-3 401 classifies as auth failure, exit 1.**
Given the fake server returns 401, when ask runs, then exit `1`, auth code, recovery instruction (check key), no retry.
Fixture: `http/401.json`.

**D2-4 422 classifies as API failure, exit 1, no retry.**
Given 422, then exit `1` with a single request observed.
Fixture: `http/422.json`.

**D2-5 429 honors Retry-After then succeeds.**
Given 429 with `Retry-After` then 200, when ask runs, then exit `0` with recorded sleep ≈ Retry-After and exactly 2 requests.
Fixture: `http/429_once.json`.

**D2-6 429 exhausts its bound, exit 1.**
Given 429 on every attempt, then exit `1` with exactly 3 requests (default 2 retries) and recorded backoff 250ms, 500ms — doubling per attempt, single wait capped at 10s; `--max-retries` accepts 0–5.
Fixture: `http/429_exhaust.json`.

**D2-7 529 behaves like 429 (retry then classify).**
Fixture: `http/529_once.json`, `http/529_exhaust.json`.

**D2-8 Non-documented 5xx never retries.**
Given 500, then exit `1`, exactly 1 request.
Fixture: `http/500.json`.

**D2-9 Ambiguous transport failure is never replayed.**
Given the server accepts the request then drops the connection before responding, when the client fails, then exit `1` with exactly 1 request observed (no silent resend of a possibly-executed ask).
Verify: `go test ./internal/infra/typesafeapi/... -run TestNoReplayOnTransportDrop`.

**D2-10 Malformed 200 body is a response failure, exit 1.**
Given 200 with non-JSON or schema-invalid body, then exit `1`, response-failure code.
Fixture: `http/malformed_200.json`.

**D2-11 Non-JSON error bodies are contained.**
Given an HTML/text error body, then exit `1` and stdout carries only the stable code + recovery; raw body appears at most on the debug stderr channel.
Fixture: `http/nonjson_error.html`.

**D2-12 Timeout classifies as exit 1.**
Given a server exceeding the timeout (short injected timeout in tests), then exit `1` with JEQ_TIMEOUT.
Verify: `go test ./internal/infra/typesafeapi/... -run TestTimeout`; binary-level override uses `--timeout` (OQ-4 resolved).

**D2-13 Missing key fails pre-network.**
Given valid input and no `TYPESAFE_API_KEY`, when ask runs, then exit `1` (auth class) with the fake server counting `0` requests.
Verify: `go test ./internal/cli/... -run TestMissingKeyPreNetwork`.

**D2-14 Source readers handle `-` and files only.**
Given `--request -` / `--state-json -` with piped stdin, then content is read fully; with a TTY-like non-piped stdin and no `-`, then exit `2`, never blocking.
Verify: `go test ./internal/infra/source/... -run TestStdinSources` (injected streams).

**D2-15 Reader failure paths are stable.**
Given a missing file, an unreadable file (chmod 000), a directory path, an empty file, or non-UTF8 bytes, when read, then exit `2` with distinct stable codes, pre-network.
Fixtures: `sources/missing.txt` (absent), `sources/unreadable.txt`, `sources/empty.txt`, `sources/binary.bin`.

**D2-16 JSON renderer is deterministic.**
Given the same response rendered twice, then bytes are identical (no timestamps, map-order or locale variance), document + single `\n`.
Verify: `go test ./internal/infra/render/... -run TestRenderDeterminism` (render twice, byte-compare).

**D2-17 Every error names its way out (motto end-to-end; FINAL ADR contract).**
Given any failure, when the command exits non-zero, then stdout carries exactly one structured document in the selected format: exit 2 → `JEQ_INPUT_INVALID`-class code with offending input + valid alternatives; exit 1 → the matching code with a recovery instruction — never a bare code, never raw dependency prose. stderr holds diagnostics/retry progress only. **The D0 bootstrap stderr prose fallback (ff1db92/650f0a2) must be removed by this check.**
Verify: `go test ./internal/cli/... -run TestUnknownFlagSuggestions`; binary probes capture stdout+stderr for one exit-2 and one exit-1 scenario, asserting single-document stdout and quiet stderr.

**D2-18 ask end-to-end, composed mode.**
Given `--questions q.json --state "..."` and `TYPESAFE_BASE_URL` pointing at the fixture server, when run, then request body matches the composed native document, stdout matches `render/json_200_full.golden`, exit `0`.
Fixtures: `requests/composed_questions.json`, `contract/response_200_full.json`.

**D2-19 ask end-to-end, passthrough in both modes.**
Given `--request native.json` or `--questions/--state-json` documents containing unknown fields, when run, then the server receives the documents byte-preserved (unknown fields intact in both modes per OQ-1) and exit `0`.
Fixtures: `requests/native_with_unknown_fields.json`, `requests/composed_with_unknown_fields.json`.

## Resolved questions (recorded in task files, commit 92926aa)

- **OQ-1** → TASK-0003: unknown fields pass through in both modes; strictness = duplicate keys + known-field types only. (D1-3, D2-19 updated.)
- **OQ-2** → TASK-0004: local rules are exactly the client-owned invariants; server is semantic authority (422 → exit 1). (D1-7 updated.)
- **OQ-3** → TASK-0007: default 2 retries (3 attempts max), `--max-retries` 0–5, 250ms base doubling, single wait capped 10s. (D2-6 updated.)
- **OQ-4** → existing `--timeout` flag; no new knob. (D2-12 updated.)
- **OQ-5** → JSON remains the version 1 default and only supported output; TOON was deferred by TASK-0009 pending current-spec conformance.
- **OQ-6** → `models`/`version`/home-view accepted in D3; out of D2 acceptance scope.
- **OQ-7** → code format `JEQ_<AREA>_<REASON>`, registry frozen in TASK-0002. (D0-3 updated.)
