# TASK-0026 executable code-smell example verification

Independent verification of `f0f5166` (`feat: add executable code smell review example`).

## Verdict

**PASS.** The two findings from the initial review are resolved by `785deaf`.

- The successful and evaluator-failure paths now remove the mode-600 temporary source bundle.
- The README NUL-delimited Git example now uses a Bash 3.2-compatible loop.

No code was changed during this retest. Only this report amendment is intended to be committed.

## Verification evidence

### Executable script and API contract

Built the compiled binary with:

```sh
nix develop -c go build -o /tmp/gev-task0026 ./cmd/gev
```

A recording local fake API verified a successful script invocation with:

- exactly one `POST /v1/systemone` request;
- ordered state records;
- exact file content, including trailing newlines;
- a path containing a space;
- a path containing a newline;
- the six bounded primary-smell criteria;
- cohesive wording defining `high=yes` and safe to pass;
- response extras preserved under `_gev.code_smells`;
- exit status 0 and no stderr;
- no `--model` flag, an isolated empty `HOME`/`XDG_CONFIG_HOME`, and request model `jev-latest`, proving the true built-in default.

A separate exact-boundary run accepted a file of exactly **256 KiB**, sent one request, and preserved all 262,144 content bytes. A 256 KiB + 1 byte file was rejected before the API with status 2.

The fake API proves request plumbing and envelope handling only. It does not prove model quality, and the README correctly describes the review as advisory rather than a linter, proof, or replacement for deterministic checks.

### Adversarial preflight

All of these returned the expected status and made no API request where applicable:

| Input | Result |
| --- | --- |
| zero files | exit 2, usage |
| 21 files | exit 2, usage |
| missing path | exit 2, readable-regular-file error |
| directory | exit 2, readable-regular-file error |
| mode-000 unreadable file | exit 2, readable-regular-file error |
| 256 KiB + 1 byte | exit 2, size error |
| unavailable `GEV_BIN` | exit 127, explicit dependency error |
| unavailable `JQ_BIN` | exit 127, explicit dependency error |

Temporary files created by invalid preflight paths were absent. After `785deaf`, successful review, API failure, and evaluator failure all left zero `gev-code-smells.*` files in isolated `TMPDIR` directories. The created file mode remains 600.

### Questions, semantics, privacy, and cost

The embedded request was inspected and recorded:

- `primary_smell` is a bounded choice with six explicit criteria;
- `cohesive` is a `noul` question whose instructions define high as yes/safe;
- the base review is advisory and exits 0;
- the optional gate passed at 0.95 and made no API request;
- response extras were retained;
- README language states one API request per invocation, account cost, privacy/source restrictions, advisory judgment, and projection before logging/sharing.

### README examples

Under the project-supported Bash/JQ environment, these examples passed against the fake API:

- explicit-file invocation;
- `jq 'del(.items)'` safe projection while retaining evidence;
- optional offline `gev gate` with a pass decision;
- NUL-delimited Git-diff invocation, preserving all 3 selected paths.

### Bash portability

The README now uses a `while IFS= read -r -d ''` array loop instead of `mapfile`. The updated snippet was executed under macOS's default `/bin/bash` 3.2 with the TASK-0026 paths and passed, preserving all selected paths. The script itself passes `bash -n` under both Bash versions. `shellcheck` was not available in the existing Nix inputs; no dependency was added.

## Full checks

- Funzzy gen 218: **PASS** — `nixfmt --check flake.nix`, `nix flake check`, `fzz check`, build, `golangci-lint`, and `go test ./...`.
- `nix develop -c govulncheck ./...`: **No vulnerabilities found**.
- `bash -n` and `/bin/bash -n` on `review.sh`: PASS.
- `shellcheck`: not installed/available; not added.

## Findings

### Resolved R1 — successful review temporary bundle cleanup

`785deaf` removed `exec` so the `EXIT` trap runs after both successful and failed evaluator commands. The focused test `TestCodeSmellReviewCleansTemporaryBundleAfterSuccessAndFailure` passed. Manual isolated-TMPDIR checks also proved:

- success: exit 0, one API request, JSON stdout, empty stderr, zero bundles;
- API 500: exit 1 with `GEV_SERVER_ERROR` JSON stdout, empty stderr, zero bundles;
- evaluator exit 7: exit 7, empty stdout, exact `fake gev failure` stderr, zero bundles.

### Resolved R2 — macOS Bash portability

`785deaf` replaced `mapfile` with the tested Bash 3.2-compatible NUL loop. The loop passed under `/bin/bash` 3.2 and preserved paths from the Git stream.

## Final status

- Verdict: **PASS**.
- Blocking findings: **0**.
- Resolved findings: **2**.
- Code changes made during verification: **0**.
