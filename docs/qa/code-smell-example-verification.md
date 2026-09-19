# TASK-0026 executable code-smell example verification

Independent verification of `f0f5166` (`feat: add executable code smell review example`).

## Verdict

**BLOCKED: one privacy-relevant correctness defect must be fixed before acceptance.**

`examples/code-smell-review/review.sh` creates a mode-600 temporary source bundle, but its final command is `exec "$GEV_BIN" reduce ... <"$tmp"`. Replacing the shell with `exec` prevents the shell's `EXIT` trap from running. A successful review therefore leaves the complete source bundle on disk.

Observed after a successful exactly-256 KiB run with a dedicated `TMPDIR`:

```text
-rw------- ... /tmp/gev-code-smell-qa/tmp-exact/gev-code-smells.sv5IBd 262207 bytes
```

The invalid-input paths did not leak because they exit before creating the temporary file. The successful path does leak. This is material because the README explicitly warns about sending source and safe projection; leaving source behind defeats the expected privacy boundary and can consume temporary storage. The fix should preserve the trap by removing `exec` (or otherwise explicitly cleaning the file after the child exits), then add a success-path cleanup assertion to the example tests.

No code was changed during verification. Only this report is intended to be committed.

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

Temporary files created by invalid preflight paths were absent. Successful-path cleanup is the blocker described above. The created file mode was 600, which is appropriate but insufficient while the file remains.

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

The README's NUL example uses `mapfile -d ''`, which works under the project's Bash 5 environment but fails on macOS's default `/bin/bash` 3.2:

```text
/bin/bash: mapfile: command not found
```

The script itself passes `bash -n` under both Bash versions. Since the README presents the NUL snippet as a copyable command and the repository is used on macOS, the snippet should use a Bash 3.2-compatible loop instead:

```bash
changed=()
while IFS= read -r -d '' path; do
  changed+=("$path")
done < <(
  git diff --name-only -z --diff-filter=ACMR -- '*.go' '*.sh'
)
if ((${#changed[@]})); then
  ./examples/code-smell-review/review.sh "${changed[@]}"
fi
```

This replacement was executed under `/bin/bash` 3.2 with the TASK-0026 paths and passed. `shellcheck` was not available in the existing Nix inputs; no dependency was added.

## Full checks

- Funzzy gen 215: **PASS** — `nixfmt --check flake.nix`, `nix flake check`, `fzz check`, build, `golangci-lint`, and `go test ./...`.
- `nix develop -c govulncheck ./...`: **No vulnerabilities found**.
- `bash -n` and `/bin/bash -n` on `review.sh`: PASS.
- `shellcheck`: not installed/available; not added.

## Findings

### Blocking B1 — successful review leaks temporary source bundle

**Location:** `examples/code-smell-review/review.sh`, final `exec "$GEV_BIN" ... <"$tmp"`.

**Impact:** the complete source bundle survives successful execution in `TMPDIR`, despite the mode-600 permission. This violates the expected cleanup/privacy boundary.

**Required fix:** do not replace the shell before the `EXIT` trap runs, or explicitly remove the file after the evaluator exits. Add a successful-path cleanup test.

### Non-blocking O1 — README NUL example requires Bash 4+

`mapfile` is unavailable in macOS's default Bash 3.2. Replace it with the tested `while IFS= read -r -d ''` array loop above if the README is intended to be generally copyable on macOS.

## Final status

- Verdict: **BLOCKED pending B1**.
- Blocking findings: **1**.
- Non-blocking observations: **1**.
- Code changes made during verification: **0**.
