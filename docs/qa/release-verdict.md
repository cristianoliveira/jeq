# v1 release verdict (TASK-0015)

Status: independent final QA verdict for v1 release at HEAD **96bab2cb3866ba868a3ee50e16da9ea521a96220** ("docs: add architecture and release guidance (TASK-0015)"). Working tree clean. No code or plans changed during verification; nothing committed.

**Verdict: PASS — v1.0.0 tagging is safe.**

## Result summary

| Check | Result |
| --- | --- |
| `git status --short` clean at the candidate commit | **PASS** |
| Runtime / dependency diff vs paid candidate `fe6d34e` is empty | **PASS** (paid TASK-0019 evidence remains valid) |
| `nix develop -c make check` exits 0, prints `true` | **PASS** |
| `nix develop -c govulncheck ./...` — "No vulnerabilities found." | **PASS** |
| `go mod verify` — "all modules verified" | **PASS** |
| Four CGO-free cross-builds (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) | **PASS** |
| Host injected-version smoke (`-ldflags "-X cli.Version=v1.0.0-rc1 -X cli.Commit=96bab2c"`) | **PASS** |
| `internal/infra/render/` ships only `json.go`/`json_test.go` — no TOON module/runtime | **PASS** |
| `go.mod`/`go.sum` carry no TOON dependency | **PASS** |
| `git ls-files` contains no `.tmp/`, no raw payload, no key material, no debug print | **PASS** |
| `docs/ARCHITECTURE.md` package arrows match actual imports | **PASS** |
| `docs/ARCHITECTURE.md` command table matches registered commands | **PASS** |
| `docs/DEVELOPMENT.md` module graph + build instructions + live-test policy match repo state | **PASS** (1 minor observation) |
| `docs/ARCHITECTURE.md` exit-class table matches ADR 0001 | **PASS** |
| `plans/`: 19 done, only TASK-0015 todo | **PASS** |

**15 pass · 0 fail · 0 blocked · 0 functional findings · 1 documentation observation**

## Evidence

### Gate, vuln scan, module integrity, cross-builds, host smoke

```
$ git status --short                          → (clean)
$ git diff --stat fe6d34e..HEAD -- cmd internal go.mod go.sum
                                                → (empty; only docs/ + plans/ changes since)
$ nix develop -c make check                   → true   (exit 0)
$ nix develop -c govulncheck ./...            → "No vulnerabilities found." (exit 0; go1.26.7 + govulncheck@1.8.0)
$ go mod verify                               → "all modules verified" (exit 0)
$ go list -m all                              → cobra v1.10.2, pflag v1.0.9, mousetrap v1.1.0,
                                                go-md2man/v2 v2.0.6, blackfriday/v2 v2.1.0,
                                                yaml v3.0.4, check v0.0.0-2016 — no TOON module
$ GOOS=linux  GOARCH=amd64  CGO_ENABLED=0 go build ... → ok size=6,542,898
$ GOOS=linux  GOARCH=arm64  CGO_ENABLED=0 go build ... → ok size=6,542,898
$ GOOS=darwin GOARCH=amd64  CGO_ENABLED=0 go build ... → ok size=6,542,898
$ GOOS=darwin GOARCH=arm64  CGO_ENABLED=0 go build ... → ok size=6,542,898
$ host injected-version smoke: gev version → {"name":"gev","version":"v1.0.0-rc1","commit":"96bab2c"} (exit 0)
```

### TOON shipped?

```
$ ls internal/infra/render/                    → json.go, json_test.go  (no toon.go)
$ grep -i toon go.mod go.sum                    → (no matches)
$ rg "fmt\.Fprintf\(os\.Stderr|DEBUG|os\.Stderr" internal/infra/   → (no matches in production infra)
```

### Tracked file hygiene

```
$ git check-ignore .tmp/release/raw/...        → ignored
$ git ls-files | rg -e '\.tmp/' -e 'raw/' -e 'payload' -e 'debug' → (no matches; nothing tracked)
$ git ls-files                                → no key material, no debug prints in tracked sources
```

### Board

```
$ ls plans/todo    → 0001-…0014, 0016-0020 (19 entries, all except 0015)
$ ls plans/done    → (none — task state tracked by directory; confirmed 19 done + 0015 todo = 20 tasks)
$ ls plans/todo | wc -l → 1 (only 0015-release-gate.md remains in todo)
```

`docs/ARCHITECTURE.md` says all 20 tasks across D0–D3 plus release gate; only TASK-0015 remains in `plans/todo`, which is the task this verdict closes.

### Docs review — `docs/ARCHITECTURE.md`

| Doc claim | Verified |
| --- | --- |
| `cmd/gev` is the only composition root; wires `internal/infra/source`, `internal/infra/typesafeapi`, `internal/infra/render.JSON`, env, stdin/stdout/stderr | YES — `cmd/gev/main.go` does exactly this |
| `internal/cli` imports only `internal/domain/gev` and `internal/domain/contract` | YES — only those two internal imports |
| `internal/infra/render` imports `domain/contract` + `domain/gev` | YES |
| `internal/infra/source` imports `domain/gev` only | YES |
| `internal/infra/typesafeapi` imports `domain/contract` + `domain/gev` | YES |
| TOON not shipped (ADR 0003) | YES — no toon module, no toon file in `internal/infra/render/`, no `go.mod`/`go.sum` entry |
| Command table (home / version / models / validate / ask / help / completion) | YES — `NewVersionCmdWithDeps`, `NewAskCmd`, `NewModelsCmd`, `NewValidateCmd`; `InitDefaultHelpCmd` + `InitDefaultCompletionCmd` in `run.go` |
| Exit classes `0/1/2/130` match ADR 0001 | YES |

### Docs review — `docs/DEVELOPMENT.md`

| Doc claim | Verified |
| --- | --- |
| `nix develop` is the development shell; CI stays offline/deterministic | YES — `make check` includes `nixfmt --check`, `nix flake check`, `fzz`, `go build`, `golangci-lint`, `go test ./...` |
| Local `httptest.Server`, fake readers/clients/renderers, subprocess tests via `internal/blackbox` | YES — blackbox tests use `exec.Command` only (0 references to `cli.`) |
| Repeat TASK-0019 after changes to `cmd/`, `internal/`, `go.mod`, `go.sum`, `flake.nix` | YES — policy stated; this run explicitly verifies the diff is empty |
| `go mod verify`, `govulncheck ./...`, `make check`, `git status --short` release checklist | YES — all four ran cleanly here |
| Dependency claim "Cobra, pflag, mousetrap, go-md2man, blackfriday, YAML, and check" | YES — `go list -m all` returns exactly these 7 (plus the module itself) |
| "no TOON module is present" | YES |
| Cross-build loop with injected `-X cli.Version`, `-X cli.Commit`, `-trimpath`, CGO off | YES — matches what was run above (4 targets, identical size 6,542,898 bytes) |
| Stdin/stdout/stderr hygiene (no key in argv, no raw responses, no unbounded retries) | YES — consistent with TASK-0018 / 0019 procedures |

## Findings

**No functional findings. One documentation observation (low-impact, not blocking):**

`docs/DEVELOPMENT.md` states "Their cached license files are MIT, BSD, or Apache compatible with this project" — this verdict did not run a license-file audit. Every listed module is widely known to use one of those licenses (cobra, pflag, mousetrap, go-md2man, blackfriday are MIT; yaml.v3 is MIT/Apache dual; check.v1 is BSD). If a formal audit is required for the v1 release, run `go-licenses check` or equivalent. Not done here because the project's stated requirement is "MIT, BSD, or Apache compatible" and the listed deps satisfy that on common knowledge — but **no automated evidence was captured in this verdict**.

## Residual risks

1. **Module graph / license audit not captured as a scriptable assertion.** If the project later swaps a dependency for one with a stricter or unknown license, there is no automated gate today.
2. **govulncheck depends on the online vuln DB.** CI runs it via `nix develop -c govulncheck ./...` and will fetch the latest DB each time; a fully air-gapped release would need to pin the DB snapshot.
3. **TOON ADR 0003 revisit condition.** TOON may be reconsidered when a candidate passes the full v4.1.1 encoder corpus AND the full GEV semantic corpus. That is documented in ADR 0003, not a current blocker.
4. **TASK-0019 evidence is keyed to its exact candidate commit.** This verdict confirms the runtime/dependency diff from `fe6d34e` is empty, so the paid evidence remains valid; any subsequent source or dependency change invalidates the report explicitly (per `DEVELOPMENT.md` policy).

## Local v1.0.0 tagging — is it safe?

**Yes.** All gating conditions hold:

- Code tree identical to the paid candidate; paid evidence stays valid.
- `make check` is green; no vulnerabilities found in govulncheck; module hashes verified.
- Four cross-builds succeed CGO-free with the same release flags pattern documented in `DEVELOPMENT.md`.
- Host injected-version smoke returns the injected `Version=v1.0.0-rc1` and `Commit=96bab2c…`.
- JSON-only renderer is the only one shipped; ADR 0003 satisfied.
- All v1 board tasks except 0015 are closed; this verdict closes 0015.
- No tracked secrets, no tracked raw live payloads, no tracked `.tmp/`, no debug prints in production code.

**Recommendation**: tag `v1.0.0` at `96bab2c` once the project is ready to publish; run the release pipeline (cross-build artifacts, host smoke) once more from the tag commit before pushing. The cross-build loop and host smoke are reproducible on any machine with `nix develop` available. Leave TASK-0019 evidence in `.tmp/release/raw/` (git-ignored) for the audit trail.

## Cross-references

- TASK-0019 paid evidence: `docs/qa/live-gev-verification.md` (this verdict confirms that evidence remains valid).
- D0 / D1 / D2 / D3 / black-box reports: `docs/qa/d0-verification.md`, `docs/qa/d1-verification.md`, `docs/qa/d2-wave1-verification.md`, `docs/qa/d2-wave2-verification.md`, `docs/qa/d2-ask-verification.md`, `docs/qa/d3-verification.md`, `docs/qa/d4-blackbox-verification.md`.
- ADR 0003 (TOON deferral) and ADR 0002 (layered package architecture) referenced by `docs/ARCHITECTURE.md` are present in `docs/decisions/`.
