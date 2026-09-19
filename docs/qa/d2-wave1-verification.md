# D2 wave-1 verification report (TASK-0006/0008/0010 vs acceptance plan)

Status: independent QA verification + one paid production run through the real Go client. No code or plans changed; nothing committed.
Commits under test: **96718b7** (typesafeapi client), **c1da38c** (JSON renderer), **d8812e9** (source readers).
Verification environment: detached worktree at d8812e9 (`git worktree add /tmp/gev-d2w1 d8812e9`), because the main tree carries uncommitted WIP for TASK-0020 (`internal/domain/contract/empty_state_test.go`, untracked) that currently **breaks the contract-package test build** (duplicate `ruleNames` declaration) — see Findings. The D2 commits themselves are clean on the committed tree: full suite green.

## Result summary

| Check | Result |
| --- | --- |
| D2-1 bearer auth from env only, no key flag | **PASS** |
| D2-2 200 renders losslessly (golden) | **PASS** |
| D2-3 401 → auth code | **PASS** |
| D2-4 422 → API failure, sanitized detail | **PASS** |
| D2-5..7 429/529 bounded retries (Retry-After) | **DEFERRED** (TASK-0007) |
| D2-8 5xx → server error, never retried | **PASS** (classification + single-request; retry layer doesn't exist yet — full check stays with TASK-0007) |
| D2-9 ambiguous transport drop never replayed | **DEFERRED** (TASK-0007) |
| D2-10 malformed 200 → response failure | **PASS** |
| D2-11 non-JSON error bodies contained | **PASS** |
| D2-12 timeout → timeout code | **PASS** |
| D2-13 missing key fails pre-network | **PASS** |
| D2-14 stdin `-` semantics, TTY fail-fast | **PASS** |
| D2-15 reader failure paths stable | **PASS** |
| D2-16 renderer deterministic, exact newline | **PASS** |
| D2-17..19 end-to-end ask / unknown-flag doc / passthrough e2e | **DEFERRED** (TASK-0011 wiring; TASK-0017 owns the error-doc contract) |

**11 pass · 0 fail · 5 deferred (TASK-0007 ×3, TASK-0011/0017 ×2) · 2 findings (1 process, 1 low)**

## Evidence

### Local httptest suites (committed tree, d8812e9)

```
$ go test ./internal/...  → all 9 packages ok (arch, cli, contract, gev, fixtures, render, source, typesafeapi)
$ go test ./internal/infra/typesafeapi/... -v
  PASS TestEvaluateHitsSystemOneWithBearerAndContentType   PASS TestModelsHitsModelsWithBearer
  PASS TestMissingKeyFailsPreNetwork                       PASS TestStatusClassification
  PASS Test422SurfacesSanitizedServerDetail                PASS TestOversizeReplyRejected
  PASS TestErrorBodyReadBounded                            PASS TestTimeoutClassifiesAsTimeout
  PASS TestConnectionFailureClassifiesAsNetworkError       PASS TestSecretNeverLeaksIntoErrors
  PASS TestResponseBodyAlwaysClosed
$ go test ./internal/infra/render/... -v    → 8/8 PASS (golden, determinism, one-doc-one-newline, round-trip, error-doc fields, escaping stays one line, empty optionals)
$ go test ./internal/infra/source/... -v    → 8/8 PASS (exact bytes within limit, failures, unreadable, piped, TTY fail-fast pre-read, oversize, empty-forbidden, exactly-at-limit)
```

### Code inspection (requested focus points)

- **Body closure/bounds**: `defer resp.Body.Close()` on every path; success bodies read via `io.LimitReader(limit+1)` (8 MiB default) with deterministic oversize rejection (`GEV_RESPONSE_INVALID`); error bodies bounded to 64 KiB (`TestErrorBodyReadBounded`, `TestOversizeReplyRejected`).
- **Key redaction**: `sanitizeDetail` extracts only JSON `message`/`detail`/`error` string fields, truncates to 200 chars, and `strings.ReplaceAll(secret, "[redacted]")`; non-JSON error bodies produce **no** detail at all (`TestSecretNeverLeaksIntoErrors`). Error docs expose only code/message/recovery — cause unreachable from the renderer.
- **Status classification**: 401→`GEV_AUTH_REJECTED` · 422→`GEV_REQUEST_REJECTED` (new registry code, golden updated in-commit — contract discipline held) · 429/529→`GEV_RATE_LIMITED` · ≥500→`GEV_SERVER_ERROR` · timeout→`GEV_TIMEOUT` · connect→`GEV_NETWORK_ERROR`. `exit.go` maps the new code to exit 1. Missing key fails **before request construction** (`TestMissingKeyFailsPreNetwork`, server count 0).
- **Deterministic JSON + exact newline**: renderer writes the response's lossless self-encoding + exactly one `\n` (`TestRenderSuccessOneDocumentOneNewline`, `TestRenderSuccessIsDeterministic`, escaping test proves newlines inside strings can't split the document).
- **Unknown-field round trip**: `TestRenderSuccessRoundTrips` over `response_unknown_fields.json` — extras survive decode→render.
- **Source read/TTY/dual-stdin semantics**: `ReadStdin` fails fast **before any read** when `isTTY()` (never blocks), oversize bounded, empty forbidden; `ReadFile` distinguishes missing / unreadable / directory / empty / oversize with stable codes and named paths; directories detected pre-read. Dual-stdin ambiguity cannot arise: stdin is only read for explicit `-` (source matrix D1-9 governs flag conflicts).

## Paid production run — through `internal/infra/typesafeapi.Client`

Harness: `go run ./.tmp/d2live/main.go` inside the module (imports the internal client; no curl, no separate HTTP stack). Key read from env inside the process; never printed/argv/committed; leak-scanned raws clean. Synthetic state; pinned `jev-1.13.0`; one POST batching Noul+Choice+Score; one GET models. Raw client outputs: `.tmp/d2live/raw_systemone_client.json`, `raw_models_client.json` (git-ignored).

```
Run (UTC): 2026-09-19T09:12:53Z
POST /v1/systemone → 200 (implicit: decoded without coded error), 699 ms
GET  /v1/models    → 200 (implicit), 232 ms, 2 models (aliases)
Resolved model: jev-1.13.0 (== pin)
Usage: input 462 / output 73 tokens
Approx input cost: ≈ $0.0000194  ($42/Btok, output free — https://docs.typesafe.ai/models)
CHECKS passed=20/20 (schema/ranges only; exact answers withheld)
```

**Cross-check against docs/qa/live-api-baseline.md (curl pass, 08:26 UTC):** identical request document → **identical token accounting (462/73)**, same resolved model, same schema. This is itself evidence the Go client composes a byte-semantically equivalent request document. Latencies (699/232 ms vs 724/593 ms) are single samples — not comparable benchmarks.

## Findings

**F-D2-1 (process, urgent for Dave): uncommitted TASK-0020 WIP breaks the contract test build in the main tree.** `internal/domain/contract/empty_state_test.go` (untracked) declares `ruleNames` twice — `go test ./internal/domain/contract/...` fails to build at the working tree. It addresses my D1 finding F-D1-1 (whitespace-sensitive empty-state detection) — good — but as-is it turns every local run red. Commit only after `go test ./internal/...` is green; `make check` would catch it.

**F-D2-2 (LOW, deferred by design): D2-13 verified at client level only.** Missing-key pre-network behavior is enforced inside `typesafeapi.call`; when the ask command (TASK-0011) wires credential resolution, the same guarantee must be re-asserted at the CLI layer (env lookup before any file reads). Tracked here so it isn't lost; no action now.

## Corrections / cross-references

- D2-5..7, D2-9 deferred to TASK-0007 (bounded retry policy, Retry-After, no-replay-on-transport-drop). The client currently makes exactly one attempt per call — the no-replay property holds trivially today and must be re-proven when retries land.
- D2-17..19 blocked on TASK-0011 (ask e2e) and TASK-0017 (structured stdout error docs; D0 bootstrap stderr fallback still in place per PO ruling).
- D1 finding F-D1-1 is being addressed by TASK-0020 WIP (see F-D2-1).
