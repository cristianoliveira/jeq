# D2 wave-2 verification report (TASK-0007/0017 vs acceptance plan)

Status: independent QA verification — evidence from the tree and the built binary. No code or plans changed; nothing committed.
Commits under test: **0411651** (bounded retry policy, TASK-0007), **0a0cdd0** (structured unknown-command diagnostics, TASK-0017). HEAD at verification: 16d31ee (plans edit only). Working tree clean.

## Result summary

| Check | Result |
| --- | --- |
| D2-5 429 + Retry-After honored → success | **PASS** |
| D2-6 429 exhausts bound → exit 1 | **PASS** |
| D2-7 529 behaves like 429 | **PASS** |
| D2-8 non-documented 5xx never retried | **PASS** |
| D2-9 ambiguous transport drop never replayed | **PASS** |
| D2-17 unknown command / misspelling / unknown flag → structured stdout; help exit 0 | **PASS** |

**6 pass · 0 fail · 0 deferred · 2 follow-up items**

## Evidence

### D2-5..9 — Retry policy (TASK-0007) — PASS

Local httptest suite (recordingSleeper, no real waits):

```
$ go test ./internal/infra/typesafeapi/... -run 'TestRetry' -v
  PASS TestRetryOn429ThenSucceed             (1 retry → 200; recorded sleeps)
  PASS Test429ExhaustsBound                  (3 attempts on 429, exit via GEV_RATE_LIMITED)
  PASS Test529RetriesLike429                 (529 treated identically)
  PASS TestZeroRetriesMeansSingleAttempt     (1 attempt; no recorded sleep)
  PASS TestMaxRetriesClampedToFive           (6 requests with MaxRetries=99 → bounded to 5)
  PASS TestRetryAfterSecondsHonored          (delta-seconds parsed, used when ≤ 10s cap)
  PASS TestRetryAfterHTTPDateHonored         (HTTP-date parsed against injected clock)
  PASS TestRetryAfterFallbacks               (empty / malformed / cap-exceeding → backoff)
  PASS TestNeverRetryOtherStatusesOrTransport (401/422/500/connect-fail → exactly 1 request)
  PASS TestRetryDiagnosticsGoToHook          (one Diagnostic line per retry attempt)
ok  github.com/cristianoliveira/gev/internal/infra/typesafeapi
```

Code inspection confirms the contract:

- **Bounded** — `DefaultMaxRetries = 2`, `MaxRetriesLimit = 5`, `retryBaseBackoff = 250ms`, doubling per attempt, single wait capped at `retryWaitCap = 10s`. Defaults enforced in `New`; the call site clamps to `[0,5]` defensively.
- **Retry-After parsing** — `parseRetryAfter` accepts delta-seconds and HTTP-date (vs injected `Now()`), rejects malformed/empty/exceeds-cap, falls back to the doubling backoff schedule. `source` is one of `"retry-after"` or `"backoff"` and is carried into the diagnostic line for verifiability.
- **Request counts** — default 429 → 3 requests (1 + 2 retries); `MaxRetries=99` → 6 requests (1 + 5); 401/422/500/connect-fail → exactly 1.
- **Recorded sleep sequence** — `recordingSleeper.Sleep` records `time.Duration`s; no real `time.Sleep` runs in tests.
- **Diagnostic channel** — `Diagnostic(string)` injected; one line per retry attempt (format: `retry N/M after WAIT (source; status S)`). The call site **never** writes to stdout/stderr directly; the renderer remains the single stdout channel.
- **Cancellation** — before sleeping, `ctx.Err()` is checked; on cancellation the call returns `GEV_INTERRUPTED` with the canceled cause (no further attempts).
- **Body closure** — `attemptOnce` closes the response body via defer on every path; transport-error paths return immediately **without retry** — the design comment is explicit: a sent request may already have executed, so retries are unsafe. **No transport replay.** This is the D2-9 guarantee.
- **Sleep/Now/Diagnostic are all injected**, keeping the policy deterministic in tests.

### D2-17 — Structured usage errors (TASK-0017) — PASS

```
$ go build -o /tmp/gev-w2 ./cmd/gev                       → build=ok
$ nix develop -c make check                               → true
```

Streams-separated probes:

| probe | exit | stdout (one JSON doc + newline) | stderr |
| --- | --- | --- | --- |
| `gev badsubcommand` | **2** | `{"code":"GEV_INPUT_INVALID","message":"unknown command \"badsubcommand\"","recovery":"run 'gev --help' for the command list"}` | empty |
| `gev versionn` | **2** | `{"code":"GEV_INPUT_INVALID","message":"unknown command \"versionn\"","recovery":"did you mean \"version\"? run 'gev --help' for the command list"}` | empty |
| `gev --nope` | **2** | `{"code":"GEV_INPUT_INVALID","message":"unknown flag: --nope","recovery":"run 'gev --help' for the flag list"}` | empty |
| `gev --help` / `gev help` | **0** | Cobra help prose on stdout | empty |

All four failure cases produce exactly **one** parseable JSON document (`jq -e .` accepts), **one** trailing newline, and **empty stderr**. The misspelling path uses `root.SuggestionsFor(typed)` to surface the closest deterministic alternative as the recovery value (TASK-0017 golden list satisfied — unknown command, misspelled-suggestion, unknown flag, help). Help exits 0 and stays as prose on stdout, which is the documented final-criterion shape for help in D0-8.

### Interim Cobra stderr prose is GONE — confirmed

`internal/cli/root.go` sets `SilenceErrors: true` and `SilenceUsage: true`; `internal/cli/run.go` registers default help/completion, builds the structured document on `Find` failure and on Cobra-execute error (via `errors.As(err, &usage)`), renders it through the injected `Renderer`, and returns the exit code. Stream probes above show stderr empty in **all** cases including the previously-prose paths; no duplicate channels anywhere. Golden fixtures (`usage_unknown_command.golden`, `usage_suggestion.golden`, `usage_unknown_flag.golden`) live under `internal/fixtures/render/` and are byte-matched by `TestUsageFailuresAreStructuredAndStderrEmpty` (PASS).

### Gate

```
$ nix develop -c make check → true
```

## Findings

**F-D2-3 (follow-up, TASK-0011): CLI `--max-retries` flag is not wired yet.** The client clamps defensively (`c.MaxRetries` forced to `[0,5]`), but no `--max-retries` flag exists in the command tree (`rg "max-retries" cmd internal/cli` returns only the client/retry-test mentions). Implication: every CLI invocation today uses `DefaultMaxRetries = 2` regardless of what the caller wants. The CLI flag must (a) reject values outside `0..5` with a usage failure (`GEV_INPUT_INVALID`, exit 2), and (b) accept `-1`, `6`, `"foo"` etc. as flag parse errors (Cobra → exit 2). Defensive clamping is not the contract; it only prevents an unbounded retry if some future caller bypasses validation. Owner: Dave, TASK-0011 follow-up. Not fixable from these two commits.

**F-D2-4 (informational, no action): help output is prose, not JSON.** Cobra's help text on stdout is correct per the D0-8 final criterion ("help stays exit 0 on stdout") and per the TASK-0017 goldens (`help` exit 0 is part of the golden list). Recording for the audit trail so a future check doesn't conflate "every exit is one JSON doc" with "every stdout is one JSON doc" — help prose is the documented exception.

## Cross-references

- D2-1..16 already accepted in docs/qa/d2-wave1-verification.md (wave 1: 96718b7/c1da38c/d8812e9).
- D0-8 final criterion (structured stdout doc, stderr quiet) is now satisfied at the binary level for usage failures; F-1 is fully closed. The bootstrap fallback (ff1db92/650f0a2) is no longer reached on the failure path.
- D2-19 (e2e unknown-field passthrough) still requires TASK-0011 ask wiring — re-verify when the ask command lands.
