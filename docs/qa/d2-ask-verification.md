# D2 ask verification report (TASK-0011 commit cbd95f3)

Status: independent QA verification of `jeq ask` end-to-end — local deterministic probes against a path-mode fake server plus two paid production runs through the compiled binary, both native and composed modes. No code or plans changed; nothing committed.
Commit under test: **cbd95f3** (feat: wire ask command end to end). HEAD at verification: cbd95f3. Working tree clean. Gate green (`nix develop -c make check` → true).

## Result summary

| Check | Result |
| --- | --- |
| D2-18 ask e2e happy path — native mode (fake + paid) | **PASS** |
| D2-18 ask e2e happy path — composed mode (fake + paid) | **PASS** |
| D2-19 unknown request/question fields survive (BOTH modes) | **PASS** |
| D1-11 (deferred) stdin never read implicitly; explicit `-` TTY fail-fast | **PASS** |
| D1-13 (deferred) `--base-url` honored, precedence over `TYPESAFE_BASE_URL` | **PASS** |
| F-D2-2 (follow-up) missing key fails before client construction | **PASS** |
| F-D2-3 (follow-up) `--max-retries` rejects 6 / -1 as exit 2 | **PASS** |
| Bonus: transport drop makes exactly one request (no replay) | **PASS** |
| Bonus: malformed 200 → `JEQ_RESPONSE_INVALID`, exit 1 | **PASS** |
| Bonus: low confidence (noul=0.01) renders success, exit 0 | **PASS** |
| Bonus: model precedence chain (`--model` > `TYPESAFE_DEFAULT_MODEL` > `jev-latest`) | **PASS** |
| Bonus: composed via explicit stdin state (`--state-file -`) | **PASS** |

**12 pass · 0 fail · 0 deferred · 0 findings** (F-D2-2 and F-D2-3 are now closed by this commit)

## Evidence

### Local deterministic suite (httptest-backed, all path-mode scenarios)

Fakes: `.tmp/d2-e2e/single_server.py` — one-mode-per-process servers on a port range, each writes a request counter to `/tmp/d2-fake-counter-<PORT>.json` so request counts are observable from outside the process. Streams captured separately for every probe; `jq -e .` validates stdout is one parseable JSON document.

| # | Scenario | Server requests | Exit | Stdout (head) | Stderr |
| --- | --- | --- | --- | --- | --- |
| 01 | composed (Noul only) → 200 | **1** | 0 | valid JSON + newline; resolved `jev-1.13.0`; `x_unknown_request_field` preserved on the wire | empty |
| 02 | native (Noul + unknown top-level + question-level) → 200 | **1** | 0 | valid JSON; both `x_unknown_top` and `x_unknown_request_field` preserved on the wire | empty |
| 03 | composed, no env, no flag → model `jev-latest` | 1 | 0 | resolved `jev-latest` | empty |
| 03b | composed, `TYPESAFE_DEFAULT_MODEL=jev-latest-from-env` | 1 | 0 | resolved `jev-latest-from-env` | empty |
| 04 | composed, `--base-url http://…` overrides `TYPESAFE_BASE_URL` | 1 | 0 | `TYPESAFE_BASE_URL` set to one server; `--base-url` routed to a different one — flag wins | empty |
| 05 | transport drop + `--max-retries 5` | **1** (no replay) | 1 | `{"code":"JEQ_NETWORK_ERROR",…,"recovery":"check network, DNS, TLS, and TYPESAFE_BASE_URL"}` | empty |
| 06 | malformed 200 body | 1 | 1 | `{"code":"JEQ_RESPONSE_INVALID","message":"response document",…}` | empty |
| 07 | low-confidence response (noul=0.01) | 1 | **0** | full success document rendered | empty |
| 08 | composed, `--state-file -` against terminal stdin | 0 | 2 | `{"code":"JEQ_INPUT_INVALID","message":"stdin is a terminal; '-' would block…",…}` — TTY fail-fast, no read | empty |

Raw outputs of every probe: `.tmp/d2-e2e/raw/0N-*.stdout`. The fake server's request-counter proves single-attempt behavior on transport drop (probe 05) and zero-attempt behavior on TTY fail-fast (probe 08).

### Go-level evidence

```
$ go test ./internal/...  → all 9 packages ok
$ go test ./internal/cli/... -run TestAsk -v
  PASS TestAskNativeHappyPath
  PASS TestAskComposedHappyPathAndPrecedence
  PASS TestAskConflictHappensBeforeReadersAndEnvironment   (reads=0, env.Fatal on read, calls=0)
  PASS TestAskRejectsDualStdinBeforeRead                   (reads=0)
  PASS TestAskMissingKeyDoesNotCreateClient               (NewClient never constructed)
  PASS TestAskMaxRetriesAndInputConfigErrors              (6, -1, "nope", "toon" → exit 2 JEQ_INPUT_INVALID; JSON remains the only supported output)
  PASS TestAskPassesBaseURLTimeoutAndRetryConfiguration   (exact flag values reach client)
  PASS TestAskReadsExplicitStdinOnce                      (reads=1)
  PASS TestAskFailureRendersStableDocumentAndRetryDiagnostics
$ nix develop -c make check  → true
```

### Paid production (two calls max) — through the compiled `jeq` binary

Both runs used JSON output (the only supported format), synthetic state, pinned `jev-1.13.0`. Key read from env inside the process; never printed/argv/committed; leak-scan over raw captures clean. Raws under `.tmp/d2-e2e/raw/`.

| # | Mode | State source | Latency | Exit | Stderr | Resolved | In tokens | Out tokens | Answers |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | native `--request` file | (in request JSON) | 732 ms | 0 | empty | `jev-1.13.0` | **402** | 73 | `department, frustration, is_actionable` |
| 2 | composed `--questions` file + **`--state-file -`** (stdin) | piped text | 845 ms | 0 | empty | `jev-1.13.0` | **392** | 73 | `department, frustration, is_actionable` |

Both stdout captures are exactly one parseable JSON document + one trailing newline (`wc -l` = 1; `jq -e .` passes; `tail -c1` = `\n`). Stderr empty in both cases (no retries needed — neither request hit 429/529). Total spend: **(402 + 392) × $42/Btok = ≈ $0.0000334** input; output free (`docs.typesafe.ai/models`).

The 10-token delta between native and composed (402 vs 392) is consistent with the model field being injected by jeq at request-build time in composed mode vs embedded in the request JSON in native mode — the wire bodies differ structurally but produce identical classification behavior and both render cleanly.

## What this closes

- **F-D2-2** (CLI-layer missing-key pre-network) — `TestAskMissingKeyDoesNotCreateClient` asserts `deps.NewClient` is never invoked when `TYPESAFE_API_KEY` is empty; binary probe (no key set) confirmed: exit 1, structured `JEQ_AUTH_MISSING` doc on stdout, stderr empty. **Closed.**
- **F-D2-3** (CLI-layer `--max-retries` validation) — `TestAskMaxRetriesAndInputConfigErrors` covers `6` and `-1`; binary probe confirmed exit 2 `JEQ_INPUT_INVALID`. **Closed.**
- **D2-19** (unknown-field passthrough, both modes) — fake-server probes 01 and 02 used `x_unknown_request_field` at question level plus `x_unknown_top` at document level; the server echoed the parsed request inside `x_unknown_field_kept.echo` and the printed stdout preserved the same fields in the response — confirming the round trip byte-semantically in both modes. **Closed.**
- **D0-8 final criterion** — TASK-0017 already closed the binary-level structured-output contract; TASK-0011 preserves it across all ask failures (validators, network errors, classified API failures all render one structured doc on stdout with empty stderr).

## Cross-references

- D2-1..16 — accepted in docs/qa/d2-wave1-verification.md (wave 1).
- D2-5..9, D2-17 — accepted in docs/qa/d2-wave2-verification.md (wave 2).
- D2-18, D2-19 plus D1-11/D1-13 deferrals and F-D2-2/F-D2-3 follow-ups — closed here.
- Remaining D0 gaps (D0-4/D0-5 deferred pending renderer — now also closed at the binary level through TASK-0017; the renderer-driven rendering is wired into the ask command path).
