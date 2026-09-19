# GEV QA acceptance plan — D0–D2 (TASK-0016)

Status: plan for review — no code, no board changes.
Ground truth: ADR 0001 (agent-first CLI contract), ADR 0002 (layered architecture), plans/todo TASK-0001..0011.
Exit-class truth from ADR 0001: `0` success · `1` auth/API/network/timeout/response failure · `2` usage or locally invalid input · `130` interrupted. Low confidence is never an error.

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
Given the code registry, when a code is added/removed/renamed, then a golden snapshot test fails until the change is explicit.
Verify: `go test ./internal/domain/gev/... -run TestErrorCodeRegistryGolden` — fixture `contract/error_codes.golden`.

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

## D1 — Contract types, validation, ask composition (TASK-0003..0005)

All D1 checks assert **exit `2` with a stable code, empty machine output, zero network requests, zero credential lookup**. Shared verify pattern: `go test ./internal/domain/... -run Test<Input>` with the named fixture; e2e variants via `go test ./internal/cli/... -run TestAskLocal`.

**D1-1 Malformed JSON fails locally.**
Given syntactically invalid JSON (truncated, trailing comma, BOM), when decoded, then exit `2`, code set, no network.
Fixture: `contract/malformed_truncated.json`, `contract/malformed_trailing_comma.json`.

**D1-2 Duplicate keys are rejected, not last-wins.**
Given a document with a duplicate object key, when strictly decoded, then exit `2` with a duplicate-key code.
Fixture: `contract/dup_key.json` (`{"state":"a","state":"b"}` shape analog in questions).

**D1-3 Unknown fields fail strict decode (composed mode).**
Given `--questions` JSON containing an unrecognized field, when decoded, then exit `2` naming the offending field. *(Scope limit vs native passthrough — see open questions OQ-1.)*
Fixture: `contract/unknown_field.json`.

**D1-4 Type mismatches fail strict decode.**
Given a probability supplied as a string or levels as a scalar, when decoded, then exit `2`.
Fixture: `contract/type_mismatch.json`.

**D1-5 Response decode is lossless.**
Given a response containing fields gev does not model, when decoded and re-rendered, then the unknown fields are preserved verbatim.
Fixture: `contract/response_unknown_fields.json`; verify: `go test ./internal/domain/contract/... -run TestLosslessResponse`.

**D1-6 Unknown primitive fails locally.**
Given a question whose primitive is not `noul|choice|score`, when validated, then exit `2` pre-network.
Fixture: `contract/unknown_primitive.json`.

**D1-7 Primitive shape rules fail locally.**
Given `choice` without options or `score` without levels, when validated, then exit `2`. *(Exact rule list — see open question OQ-2.)*
Fixtures: `contract/choice_no_options.json`, `contract/score_no_levels.json`.

**D1-8 Empty required sections fail locally.**
Given empty `questions`, or composed mode resolving to an empty state, then exit `2`.
Fixtures: `contract/empty_questions.json`, `contract/empty_state.json`.

**D1-9 Mode conflicts fail before any I/O.**
Given `--request` combined with `--questions`, or `--request` with any `--state*`, or two `--state*` flags together, when parsed, then exit `2` with the conflict code, and the fake server counted `0` requests and injected readers observed no file/env access.
Verify: `go test ./internal/domain/gev/... -run TestModeConflictMatrix` (table = the full conflict matrix from ADR 0001).

**D1-10 Exactly one state source is accepted.**
Given each of `--state`, `--state-file`, `--state-json`, `--state-json -` alone with `--questions`, when composed, then composition succeeds and proceeds to evaluation (no conflict error).
Fixture: `requests/composed_questions.json` + per-source state fixtures.

**D1-11 stdin is never read implicitly.**
Given ask invoked with explicit non-stdin inputs while stdin is closed (not `-`), when the command runs, then it completes without blocking.
Verify: `go test ./internal/cli/... -run TestNoImplicitStdin` with injected closed stream; binary smoke: `gev ask --questions q.json --state "x" </dev/null`.

**D1-12 Model resolution follows the documented chain.**
Given none/each of `--model`, `TYPESAFE_DEFAULT_MODEL`, set, when composing, then the resolved model is respectively `jev-latest` → env → flag (flag wins).
Verify: `go test ./internal/domain/gev/... -run TestModelPrecedence` with `t.Setenv`.

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
Given a recorded 200 response, when ask completes, then exit `0` and stdout is exactly the golden JSON (resolved model, every answer, probabilities, confidence, score legends, usage) plus one trailing newline.
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
Given 429 on every attempt, then exit `1` with attempt count == configured bound (value per OQ-3).
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
Given a server exceeding the timeout (short injected timeout in tests), then exit `1`, timeout code.
Verify: `go test ./internal/infra/typesafeapi/... -run TestTimeout` (override mechanism per OQ-4).

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

**D2-17 Unknown flags name valid alternatives.**
Given an unknown flag or command, when Cobra errors, then stderr identifies valid alternatives per ADR 0001, exit `2`.
Verify: `go test ./internal/cli/... -run TestUnknownFlagSuggestions`.

**D2-18 ask end-to-end, composed mode.**
Given `--questions q.json --state "..."` and `TYPESAFE_BASE_URL` pointing at the fixture server, when run, then request body matches the composed native document, stdout matches `render/json_200_full.golden`, exit `0`.
Fixtures: `requests/composed_questions.json`, `contract/response_200_full.json`.

**D2-19 ask end-to-end, native passthrough.**
Given `--request native.json`, when run, then the server receives the document byte-preserved (unknown fields intact) and exit `0`.
Fixture: `requests/native_with_unknown_fields.json`.

## Open questions (not decided here — product/contract calls)

- **OQ-1** Does strict unknown-field rejection apply to `--request` native passthrough, or only to composed mode? Passthrough implies losslessness; strictness implies rejection. Both cannot hold for unknown fields. (Surfaced in D1-3/D2-19.)
- **OQ-2** Which field-level rules does local validation enforce (e.g., required question `instructions`/`criteria` per TypeSafe docs) versus deferring to server 422? D1-7 scope depends on this.
- **OQ-3** Retry bound is unspecified in ADR 0001 ("bounded"): exact max attempts and backoff base needed to assert D2-6.
- **OQ-4** Timeout override surface: ADR fixes a 10s default but names no flag/env to change it; D2-12 tests inject internally, but binary-level verification needs a knob.
- **OQ-5** Default output in D2: ADR 0001 makes lossless TOON the default, but the TOON renderer (TASK-0009) is not in D0–D2. Is JSON the interim default until 0009 lands, and is that an acceptable temporary contract?
- **OQ-6** Are `models`/`version`/home-view client behaviors (TASK-0006 "two endpoints", TASK-0012) accepted in D2 or deferred to D3 acceptance?
- **OQ-7** Symbolic code namespace and registry format (e.g., `GEV-E-*` tokens) are referenced but not specified; D0-3 golden needs a chosen format.
