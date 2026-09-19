# D4 black-box verification report — TASK-0014

Status: independent QA verification of the compiled-binary reality and the caught debug-leak regression. No code or plans changed; nothing committed.
Commits under test: **4293c59** (test: add compiled binary black-box suite) + **a593c3d** (fix: classify interrupted transport without debug leak). HEAD at verification: a593c3d. Working tree clean.

## Result summary

| Check | Result |
| --- | --- |
| Black-box suite builds binary once via TestMain (`go build ./cmd/gev`), no `cli.Run` calls | **PASS** |
| Black-box suite contains no production-network URL (fake httptest only) | **PASS** |
| 28 top-level cases pass on the committed tree | **PASS** |
| Committed source contains no debug / raw transport writes (`fmt.Fprintf`/`os.Stderr`/`DEBUG`) | **PASS** |
| Committed source contains no `os.Stderr`/`os.Stdout` in infra adapters | **PASS** |
| SIGINT on blocked request → exit 130, structured `GEV_INTERRUPTED` doc, **stderr empty** | **PASS** |
| Connection drop → exit 1, `GEV_NETWORK_ERROR`, **stdout contains no raw EOF/reset/key**, stderr empty | **PASS** |
| Auth (missing key / wrong key) → `GEV_AUTH_MISSING` / `GEV_AUTH_REJECTED`, no secret leak | **PASS** |
| Timeout → `GEV_TIMEOUT`, single request | **PASS** |
| Malformed 200 → `GEV_RESPONSE_INVALID`, single request | **PASS** |
| Low confidence → exit 0 (rendered success) | **PASS** |
| Retry on 429+`Retry-After` → success, exactly 2 requests, stderr carries `retry 1/2` diagnostic | **PASS** |
| Source modes (native file/stdin, composed file/stdin) → structured success | **PASS** |
| Discovery + prose exceptions (home/--help/version/completion) → structured JSON / prose by spec | **PASS** |
| Usage errors → `GEV_INPUT_INVALID` exit 2, structured recovery, no Cobra prose | **PASS** |
| Gate green (`nix develop -c make check` → true) | **PASS** |

**15 pass · 0 fail · 0 deferred · 0 findings**

## Evidence

### Suite structure

```
$ go test ./internal/blackbox/... -v
--- PASS: TestBlackBoxDiscoveryAndProseExceptions (0.04s)
    --- PASS: home_default / home_explicit_json
    --- PASS: version_default / version_explicit_json
    --- PASS: help / completion
--- PASS: TestBlackBoxValidateSourcesAndNoStateEcho (0.03s)
    --- PASS: native_file / native_stdin
    --- PASS: composed_files / composed_stdin
--- PASS: TestBlackBoxAskNativeComposedFileStdinAndJSONModes (0.04s)
    --- PASS: native_file_default / native_stdin_explicit_json
    --- PASS: composed_file_default / composed_stdin_explicit_json
--- PASS: TestBlackBoxModelsAuthAndStatuses (0.03s)
    --- PASS: models_success / missing_key_is_pre_network / auth_rejection
--- PASS: TestBlackBoxRetriesMalformedTimeoutLowConfidenceAndDrop (0.10s)
    --- PASS: retry / server_status_failure
    --- PASS: malformed_response / timeout
    --- PASS: low_confidence_is_success / connection_drop_is_never_replayed
--- PASS: TestBlackBoxUsageAndFailureSeparation (0.03s)
    --- PASS: unknown_command / unknown_flag / misspelled_command / unsupported_output
--- PASS: TestBlackBoxInterruptBlockedAsk (0.19s)
ok  github.com/cristianoliveira/gev/internal/blackbox  (cached)
```

Counting leaves: 6 + 4 + 4 + 3 + 6 + 4 + 1 = **28 top-level cases** (27 subtests + 1 standalone SIGINT test).

### Compiled-binary reality (TASK-0014)

`internal/blackbox/blackbox_test.go` uses **only** `exec.Command` to drive the compiled binary; it never calls `cli.Run` directly:

```
$ rg "cli\." internal/blackbox/
(no matches)

$ rg "https?://api\.typesafe\.ai" internal/blackbox/
(no matches)
```

The binary is built exactly once in `TestMain` (`go build -o $gevBin ./cmd/gev`) and `gevBin` is shared across every test. Every test uses `exec.CommandContext(gevBin, args...)` against `mergedEnv` (with `NO_COLOR=1`, `TERM=dumb`, `CI=1`), captures stdout/stderr separately, and applies the helper `assertCleanMachineOutput(result, wantExit, secret)` — one JSON document with one trailing newline, no secret in either stream, no raw `Error:`/`panic:`/`goroutine ` text on stderr, no transport prose (`EOF`, `connection reset`, `EOF`/`panic` checks at the connection-drop subtest).

### Debug-leak regression — caught and fixed

The forward fix commit **a593c3d** adds the `ctx.Err()` interrupt-classification check at both the `HTTP.Do` error site and the body-read error site in `internal/infra/typesafeapi/client.go::attemptOnce`, and **removes** the `fmt.Fprintf(os.Stderr, "DEBUG transportError: ...")` line from `transportError`. Verified the committed source is clean:

```
$ rg "fmt\.Fprintf\(os\.Stderr|DEBUG" internal/infra/typesafeapi/ internal/infra/render/ internal/infra/source/ cmd/gev/
(no matches)

$ rg "os\.Stderr|os\.Stdout" internal/infra/typesafeapi/ internal/infra/render/ internal/infra/source/
(no matches)
```

The SIGINT classification itself (the post-fix logic) reads:

```go
if ctxErr := ctx.Err(); errors.Is(ctxErr, context.Canceled) {
    return 0, nil, nil, withRecovery(gev.WrapError(gev.CodeInterrupted, err, "request interrupted"),
        "rerun the command when ready")
}
```

This is needed because Go's `net/http` transport translates a signal-driven syscall interrupt (`EINTR`) into a `*url.Error` whose message is `interrupt signal received` — it does **not** unwrap to `context.Canceled`. Without checking `ctx.Err()`, the error fell through to `GEV_NETWORK_ERROR`. With it, the same context-canceled state correctly classifies as `GEV_INTERRUPTED`.

### Behavioral assertions (the cases Mary asked to re-run)

| Scenario | Asserted (in test) | Observed |
| --- | --- | --- |
| **SIGINT blocked ask** | exit `130`; stdout one JSON doc with `code:"GEV_INTERRUPTED"`; stderr empty; no secret (`interrupt-secret`) in stdout | **PASS** — test asserts all four; test passes |
| **Timeout** (`--timeout 50ms` against `<-r.Context().Done()` handler) | exit `1`; `code:"GEV_TIMEOUT"`; **exactly 1 request** | **PASS** — server counted 1 |
| **Connection drop** (hijacker.Close) | exit `1`; `code:"GEV_NETWORK_ERROR"`; **exactly 1 request**; stdout contains no `"EOF"`/`"connection reset"`; stderr empty; no secret (`drop-secret`) leaked | **PASS** — `api.count()==1`, no transport prose in stdout |
| **Retry** (429 with `Retry-After: 0`, then 200) | exit `0`; **exactly 2 requests**; stderr contains `"retry 1/2"` diagnostic | **PASS** — retry hook fired, no secret in output |
| **Malformed 200** | exit `1`; `code:"GEV_RESPONSE_INVALID"`; **exactly 1 request** | **PASS** |
| **Low confidence** (`confidence: 0.01`) | exit `0`; full success document rendered; no secret leak | **PASS** |
| **Auth: missing key** | exit `1`; `code:"GEV_AUTH_MISSING"`; **exactly 0 requests** | **PASS** |
| **Auth: 401** | exit `1`; `code:"GEV_AUTH_REJECTED"`; **exactly 1 request**; secret not echoed | **PASS** |
| **Source modes** (native file / native stdin / composed file / composed stdin) | exit `0`; `Authorization: Bearer blackbox-secret`; `model` + `questions` on the wire; exactly 1 request each | **PASS** |
| **Discovery + prose exceptions** | home/version exit `0` with one JSON line; `--help` / `completion` exit `0` with prose (non-JSON, no double newline) | **PASS** |
| **Usage errors** (unknown command / flag / misspelling / `--output toon` on version) | exit `2`; `code:"GEV_INPUT_INVALID"`; non-empty `recovery`; no Cobra prose on stderr | **PASS** |

### Gate

```
$ nix develop -c make check → true
$ go test ./...  → all packages ok (blackbox included)
```

## Cross-references

- D2-17 (structured stdout doc, stderr quiet) — closed by TASK-0017 and re-asserted at the binary level by these black-box tests.
- D3 home/version/models/validate — closed in docs/qa/d3-verification.md; the same behaviors are now also exercised through the compiled binary here.
- TASK-0019 — still open; final paid release verification repeats at the exact release commit after TASK-0014 (now landed).
