# Live `jeq` final verification (TASK-0019)

Status: final paid production verification at the candidate release commit. No code, plans, dependencies, or builds changed during this run; nothing committed. Live evidence under `.tmp/release/` (git-ignored).
Candidate commit: **fe6d34e6cdad13790cbc1d3286a6b8c2044a1e9d** ("plans(doing): TASK-0019 final paid production verification to Kelly"). Tree clean at start and end.
Toolchain: **go1.26.0 darwin/arm64** (Darwin 25.6.0, Apple silicon).

## Result summary

| Run | Result |
| --- | --- |
| A — no-arg `jeq` (offline home) | **PASS** |
| B — `jeq version` with injected linker values | **PASS** |
| C — `jeq models` (real production) | **PASS** |
| D — `jeq validate` with `TYPESAFE_API_KEY` removed (offline proof) | **PASS** |
| E — `jeq ask --request` (default JSON, no `--output`) | **PASS** |
| F — `jeq ask --request --output json` (same request) | **PASS** |
| G — invalid-key ask, isolated child process → `JEQ_AUTH_REJECTED` | **PASS** |
| Default-vs-explicit JSON structural contract | **PASS** |
| Secret-leak scan over all raw captures | **PASS** (overall clean) |
| Gate at candidate commit (`nix develop -c make check`) | **PASS** |

**10 pass · 0 fail · 0 deferred · 0 findings**

## Tooling

- Binary: built from candidate commit with `go build -trimpath -ldflags "-X .../cli.Version=v1.0.0-rc1 -X .../cli.Commit=fe6d34e6cdad13790cbc1d3286a6b8c2044a1e9d" -o /tmp/jeq-release ./cmd/jeq`. SHA-256 `96fe50462b3b49300c2a71f4fec2ae2545cc4f668bfe13a7fd947dd257f76a2e` · 9 511 474 bytes.
- All seven runs drive the compiled binary directly; the captured raw outputs live under `.tmp/release/raw/`.
- Cost basis: `$42 per Btok input`, output free (`https://docs.typesafe.ai/models`, retrieved 2026-09-19).

## Evidence per call

### A — no-arg home (offline)

```
$ unset TYPESAFE_API_KEY TYPESAFE_BASE_URL TYPESAFE_DEFAULT_MODEL
$ /tmp/jeq-release >o.txt 2>e.txt
exit=0  lines=1  trailing-newline=\n  valid-json=yes  stderr=[]
identity,credential_ready,default_model = jeq,false,jev-latest
commands = ask,models,validate,version,help
no-secret-leaked: confirmed
```

### B — version (linker-injected values surface)

```
$ /tmp/jeq-release version >o.txt 2>e.txt
exit=0  lines=1  trailing-newline=\n  valid-json=yes  stderr=[]
name,version,commit = jeq,v1.0.0-rc1,fe6d34e6cdad13790cbc1d3286a6b8c2044a1e9d
```

### C — `jeq models` (real production)

```
$ /tmp/jeq-release models >o.txt 2>e.txt
exit=0  lines=1  trailing-newline=\n  valid-json=yes  stderr=[]
models_count=2  model_names=jev-latest,jev-preview
```

### D — `jeq validate` with `TYPESAFE_API_KEY` removed (offline)

```
$ unset TYPESAFE_API_KEY TYPESAFE_BASE_URL TYPESAFE_DEFAULT_MODEL
$ /tmp/jeq-release validate --questions q.json --state-json sj.json --model jev-latest >o.txt 2>e.txt
exit=0  stderr=[]
valid=true, mode=composed, model=jev-latest, question_count=3
state-leaked=0 (must be 0)   secret-leaked=0 (must be 0)
```

### E — `jeq ask` default output (no `--output`) — native mode, pinned jev-1.13.0

```
exit=0  lines=1  trailing-newline=\n  valid-json=yes  stderr=[]
resolved_model = jev-1.13.0                (== pin)
input_tokens 388   output_tokens 73        (388 × $42/Btok ≈ $0.0000163)
answers_keys = department,frustration,is_actionable
choice_sum = 1.00                          (0 + 0 + 1.00)
score_sum  = 1.00                          (0.05 + 0.94 + 0.01)
noul_in_range = true · choice_confidence_in_range = true · score_confidence_in_range = true
score_value_in_range = true (0.95 ∈ [0, 2])
score_legend_levels = 3
```

### F — `jeq ask … --output json` (same request) — structural contract preserved

```
exit=0  lines=1  trailing-newline=\n  valid-json=yes  stderr=[]
resolved_model = jev-1.13.0   input_tokens = 388   output_tokens = 73
```

**Structural comparison** (default vs explicit, computed):

```
default keys  = ['answers', 'model', 'usage']
explicit keys = ['answers', 'model', 'usage']
identical-shape = True
same resolved_model = True
same in_tokens  = True
same out_tokens = True
```

Per-answer probability values naturally differ across the two calls (same model, same prompt → independent samples). Exact judgments are not asserted; only schema/ranges/sums/tokens are. The structural contract — top-level keys, renderer shape, line count, trailing newline, stderr empty — is identical.

### G — invalid-key ask, isolated child process, fake 401

```
$ ( export TYPESAFE_API_KEY=intentionally-bogus-key-12345 \
    TYPESAFE_BASE_URL=http://127.0.0.1:18202; \
    /tmp/jeq-release ask --request shared.json >o.txt 2>e.txt )
exit=1  stderr=[]
stdout = {"code":"JEQ_AUTH_REJECTED",
          "message":"the server rejected the credential: unauthorized",
          "recovery":"check TYPESAFE_API_KEY; it is missing, revoked, or mistyped"}
```

The bogus key never reached this shell's environment. The fake server returned `{message:"unauthorized"}`; jeq extracted only that string into the sanitized message field — no raw response body leak, no stack trace, no credential value, no transport prose. Exit 1, exact code, structured doc.

## Total spend

- `models` — GET /v1/models is not billed by the documented pricing model (no `usage` field returned).
- `ask` ×2 — **388 × $42/Btok** ≈ **$0.0000163 each** · **$0.0000326 total** (input only; output free).
- `validate`, `version`, `home`, bad-key `ask` — all local; zero cost.

## Gate at the candidate commit

```
$ nix develop -c make check → true   (exit 0)
$ git status --short → (clean)
$ git diff HEAD~1 HEAD -- go.mod go.sum → (no changes since last QA pass)
```

## Findings

None. One implementation note worth recording for the audit trail:

**Source/dependency state at the moment of verification.** `git status` was clean before the run, `go.mod`/`go.sum` were unchanged through and after the run, the gate passed once at the candidate commit, and no files were added or modified by the harness. If anything changes in `cmd/`, `internal/`, `go.mod`, `go.sum`, or `flake.nix` after this report is filed, **the live evidence must repeat at the new exact commit** per the candidate-commit requirement.

## Cross-references

- TASK-0014 (black-box suite) — verified in docs/qa/d4-blackbox-verification.md; the suite is part of the gate that turned green here.
- D3 home/version/models/validate/default-JSON — verified in docs/qa/d3-verification.md; this run re-exercises them through the release-shaped binary.
- D2 ask (default-JSON, native + composed) — verified in docs/qa/d2-ask-verification.md; this run re-runs the native default-JSON path through the release binary.
- D1 contract/types/validation — verified in docs/qa/d1-verification.md.
- D0 foundation + error codes — verified in docs/qa/d0-verification.md.
