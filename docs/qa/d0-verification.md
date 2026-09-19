# D0 verification report (TASK-0001/0002 vs acceptance plan)

Status: independent QA verification, nothing committed, no code changed.
Verified against: `docs/qa/acceptance-d0-d2.md` (D0 section).
Commit under test: **83fc08d**. Code equivalence evidence: `git diff --stat 83fc08d HEAD -- . ':(exclude)plans' ':(exclude)docs'` is empty — commits after 83fc08d touch plans/docs only, so verification at HEAD (af7de85) is verification at 83fc08d.

## Result summary

| Check | Result |
| --- | --- |
| D0-1 normal gate | **PASS** |
| D0-2 exit-class mapping | **PASS** |
| D0-3 stable code registry | **PASS** |
| D0-4 error document shape | **DEFERRED** (render path not built) |
| D0-5 redaction | **DEFERRED** (render path not built) |
| D0-6 interruption → 130 | **PASS** (mapping level) |
| D0-7 confidence never an error | **PASS** (mapping level) |

**5 pass · 0 fail · 2 deferred · 1 finding**

## Evidence

### D0-1 Normal gate is green and enforcing — PASS

```
$ nix develop -c make check
true
exit=0
```

Gate covers nixfmt/flake check, fzz, `go build ./...`, `golangci-lint run`, `go test ./...` (including `internal/arch` import-rule tests per ADR 0002). Observation (environmental, not a gap): bare `make check` outside the devshell stops at `nixfmt: No such file or directory` (Error 127); the canonical invocation is inside `nix develop`.

### D0-2 Exit-class mapping is total and deterministic — PASS

```
$ go test ./internal/cli/... -run 'TestExitCodeMapping|TestEveryStableCodeHasAnExitClass' -v
--- PASS: TestExitCodeMapping (23 cases incl. wrapping and precedence)
--- PASS: TestEveryStableCodeHasAnExitClass
ok  github.com/cristianoliveira/jeq/internal/cli
```

Binary confirmation:

```
$ /tmp/jeq-verify badsubcommand; echo exit=$?    → exit=2
$ /tmp/jeq-verify --nope; echo exit=$?           → Error: unknown flag: --nope / exit=2
$ /tmp/jeq-verify version; echo exit=$?          → {"name":"jeq","version":"dev","commit":"unknown"} / exit=0
```

`version` output verified as exactly one line (`wc -l` = 1) and one valid JSON document (`jq -e .`). Class table matches ADR 0001: 0 success · 1 API-side (7 codes + default) · 2 usage (3 codes + shell usage) · 130 interrupted. `TestRunExitCodesEndToEnd` additionally passes version/badsubcommand/--nope through `cli.Run` with injected streams.

### D0-3 Symbolic codes are a stable registry — PASS

```
$ go test ./internal/domain/jeq/... -run 'TestErrorCodesGoldenSnapshot|TestCodesRegistry|TestErrorCodeFormat' -v
--- PASS: TestErrorCodesGoldenSnapshot
--- PASS: TestErrorCodeFormat
--- PASS: TestCodesRegistry
ok  github.com/cristianoliveira/jeq/internal/domain/jeq
```

Golden snapshot at `internal/fixtures/contract/error_codes.golden` contains the 11 contractual codes; snapshot test fails on any add/remove/rename and supports explicit regeneration (`-update`). Format test enforces `JEQ_<AREA>_<REASON>` per TASK-0002.

### D0-4 Error documents follow the output contract — DEFERRED

Blocking package: `internal/infra/render` (ADR 0002; created by TASK-0008, D2) — directory does not exist yet — plus the error-document emission path in `internal/cli` (no error renderer is wired into `Run`; failures currently return exit codes without emitting a structured document). No fixture `contract/error_shape.golden` exists to test against.

### D0-5 Errors never leak secrets or internals — DEFERRED

Same blocking packages as D0-4 (`internal/infra/render` + `internal/cli` error-document emission). With no rendered error document there is nothing to redact-test; the D1/D2 acceptance checks (D1-1…, D2-10/11) inherit this obligation.

### D0-6 Interruption maps to 130 — PASS (mapping level)

```
$ go test ./internal/cli/... -run TestExitCodeMapping -v
--- PASS: TestExitCodeMapping/interrupted_code              (130)
--- PASS: TestExitCodeMapping/canceled_context_is_interrupt (130)
--- PASS: TestExitCodeMapping/interrupt_survives_wrapping   (130)
--- PASS: TestExitCodeMapping/deadline_is_not_an_interrupt  (1)
```

A stable `JEQ_INTERRUPTED` code and `context.Canceled` both map to 130, through wrapping; `DeadlineExceeded` stays class 1. Note: a real SIGINT binary probe is impossible at D0 (no long-running command exists; `version` exits immediately). Signal-level 130 verification lands with D2/0014 (black-box suite).

### D0-7 Confidence never changes exit class — PASS (mapping level)

Evidence: `ExitCode` (internal/cli/exit.go) dispatches exclusively on stable codes, `UsageError`, and `context.Canceled` — no probability/confidence input exists in the signature or table; `TestEveryStableCodeHasAnExitClass` proves the mapping covers exactly the 11-code registry and nothing else. Full-strength re-check arrives with D1 contract types (a rendered low-confidence response fixture will exercise the end-to-end path).

## Findings (contract gaps — reported, not fixed)

**F-1 — Unknown command produces a silent exit 2.**
ADR 0001 § Errors: "Unknown commands and flags identify valid alternatives." Observed on the built binary:

```
$ ./jeq badsubcommand 2>err 1>out; echo exit=$?
exit=2    stderr=[]  stdout=[]
```

Cause (code read, not modified): `internal/cli/run.go` calls `root.Find(args)` and returns `ExitCode(NewUsageError(err))` before `root.Execute()`, so Cobra never prints `Error: unknown command "badsubcommand" for "jeq"` / `Run 'jeq --help'...`. The unknown-*flag* path does print (`Error: unknown flag: --nope`). Exit code is correct; the message contract is violated — an agent gets no discovery information at all. Severity: medium; directly affects D2-17 ("Unknown flags name valid alternatives") when it runs. Suggested owner: Dave, D1/D2 window.

Update (contract correction applied; superseded in part by the Final PO ruling below): the error motto stands — non-zero is never silent — but the channel is **stdout**: ADR 0001 requires the selected structured error document on stdout; stderr stays diagnostics/retry-progress only. F-1 is therefore v1/D2 acceptance-blocking via **TASK-0017** (depends on TASK-0008 renderer, blocks TASK-0011) — and D0 remains accepted as delivered. Re-verification criteria (final, unchanged): both probes (`badsubcommand`, `--nope`) exit 2 with exactly one structured stdout document (code, offending input, valid alternatives, one trailing newline), no raw Cobra prose, stderr quiet.

Re-verification (interim fix ff1db92 + 650f0a2): silence is gone — probes exit 2 with stderr `Error: unknown command "badsubcommand" for "jeq"` / `unknown flag: --nope` + `Run 'jeq --help' for usage.`; near-miss `versionn` yields Cobra's `Did you mean this? version`; `version` unchanged (exit 0, one JSON line); TestUsageFailuresAreNotSilent green; gate `nix develop -c make check` → true.

**Final PO ruling (phase split):** the interim stderr fallback is **kept** and approved as the D0 bootstrap behavior — development failures must not be silent in the meantime. D0-8 therefore **PASSES at the bootstrap criterion** (discoverable stderr, offending input named, alternatives pointer, test-asserted). The structured stdout document remains the final, unchanged criterion: TASK-0017 (depends TASK-0008, blocks TASK-0011) must replace the fallback with exactly one structured stdout error document and remove the raw Cobra prose/duplicate stderr; **D2-17 is the final ADR contract** and does not accept the fallback. F-1 status: closed at bootstrap level, open at contract level until TASK-0017.

## Observations (no action required)

- `version` reports `commit: unknown` — expected until release `-ldflags` stamping (TASK-0015).
- D0-4/D0-5 deferral is by design: the render path is D2 scope (TASK-0008).
