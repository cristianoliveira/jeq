# D3 verification report — TASK-0009/0012/0013 vs acceptance plan

Status: independent QA verification of the v1 release wave — local deterministic probes plus paid production runs through the compiled `gev` binary. No code or plans changed; nothing committed.
Commit under test: **1f8767ae** (fix: defer TOON until v4 conformance, TASK-0009). HEAD at verification: 1f8767ae. Working tree clean. Gate green (`nix develop -c make check` → true). Per instruction: **TASK-0019 not closed** — final paid release verification will repeat at the exact release commit after TASK-0014.

## Result summary

| Check | Result |
| --- | --- |
| ADR 0003 — JSON default/only; no TOON source/module/dependency; `--output toon` exits 2 with structured JSON recovery; legacy TOON unreachable | **PASS** |
| Home view (no-arg): offline, structured, no secret, zero network | **PASS** |
| Home with key present: `credential_ready: true`, no secret echoed | **PASS** |
| Version: injected `Version`/`Commit` fields, default commit `unknown`/`dev` | **PASS** |
| Validate (composed, file state-json): offline, no credential, no state echo | **PASS** |
| Validate (native, request file): offline, no credential, no state echo | **PASS** |
| Validate (composed, `--state-file -` via piped stdin): offline, no credential, no state echo | **PASS** |
| Validate source-conflict failure → structured exit 2 (`GEV_SOURCE_CONFLICT`) | **PASS** |
| Validate offline **with TYPESAFE_API_KEY removed**: exit 0, no env lookup, no network | **PASS** |
| Models without key → `GEV_AUTH_MISSING`, exit 1, no client created | **PASS** |
| Models bad `--base-url` → `GEV_INPUT_INVALID`, exit 2 | **PASS** |
| Models bad `--timeout` → `GEV_INPUT_INVALID`, exit 2 | **PASS** |
| Paid `gev models` against production: 200, parseable aliases, 1 line | **PASS** |
| Paid `gev ask` default output, composed mode, pinned `jev-1.13.0`: valid JSON, exit 0 | **PASS** |
| Fake-server semantic equality: default-output ask byte-equals `--output json` ask | **PASS** |

**15 pass · 0 fail · 0 deferred · 0 findings** (1 documentation observation, not a defect)

## Evidence

### ADR 0003 — JSON-only renderer, no TOON shipped

- `internal/infra/render/toon.go` and `toon_test.go` deleted in commit 1f8767ae (`git show --stat` confirms).
- `rg "toon|TOON" go.mod go.sum internal/infra/render/` → **no matches**; no runtime encoder, no module dependency.
- `--output toon` rejection (binary probe, streams separated):

  ```
  $ /tmp/gev-d3 ask --questions /tmp/q.json --state "x" --output toon >o.txt 2>e.txt
  exit=2  stderr=[]  stdout: valid-json
  code=GEV_INPUT_INVALID  recovery=set --output json
  ```

- Code inspection (`internal/cli/discovery.go::outputRenderer`): the validator returns `GEV_INPUT_INVALID` with recovery `"set --output json"` for any value other than `json`. There is no code path that can produce TOON bytes; the renderer has only one implementation (`internal/infra/render/json.go`).

### Home view (`gev` with no args)

```
$ unset TYPESAFE_API_KEY TYPESAFE_BASE_URL TYPESAFE_DEFAULT_MODEL
$ /tmp/gev-d3 >o.txt 2>e.txt
exit=0  stdout_lines=1  stderr=[]  stdout: valid-json
stdout=[{"identity":"gev","purpose":"agent-first TypeSafe System One client","credential_ready":false,"default_model":"jev-latest","commands":["ask","models","validate","version","help"],"next_step":"run gev ask --help"}]
no-secret-leaked: confirmed
```

With key set: `credential_ready` flips to `true`; the secret value is **never** present in stdout (grep clean). Zero network: the command never instantiates the API client — only `deps.Getenv` is consulted.

### Version

```
$ /tmp/gev-d3 version >o.txt 2>e.txt
exit=0  stdout_lines=1  stderr=[]  stdout: valid-json
name,version,commit = gev,dev,unknown
```

The injected `Version`/`Commit` package vars surface; defaults `dev`/`unknown` apply when no `-ldflags` stamping is used (release gate TASK-0015 will provide stamped values).

### Validate (offline, zero credential, no state echo)

| Inputs | Mode | Exit | Stdout (validated) | Stderr | State echoed? |
| --- | --- | --- | --- | --- | --- |
| `--questions q.json --state-json /tmp/sj.json --model jev-latest` | composed | **0** | `{valid:true, mode:composed, model:jev-latest, question_count:3}` | empty | **no** (grep for state string = 0) |
| `--request native.json` | native | **0** | `{valid:true, mode:native, model:jev-1.13.0, question_count:1}` | empty | **no** |
| `--questions q.json --state-file -` (piped stdin) | composed | **0** | `{valid:true, mode:composed, …}` | empty | **no** |
| conflict: `--request a --questions b --state x` | — | **2** | structured `GEV_SOURCE_CONFLICT` | empty | n/a |
| offline with `TYPESAFE_API_KEY` removed (production run) | composed | **0** | `{valid:true, mode:composed, model:jev-latest, question_count:3}` | empty | **no** |

The validate document intentionally omits `state` — `validateDocument` only carries `valid`, `mode`, `model`, `question_count`. The state string was never read by anything in the rendered path even though the reader does consume it for composition.

### Models command — local classification

```
unset TYPESAFE_API_KEY → exit 1, code=GEV_AUTH_MISSING  (recovery: export TYPESAFE_API_KEY)
TYPESAFE_API_KEY=fake models --base-url "not-a-url" → exit 2, code=GEV_INPUT_INVALID
TYPESAFE_API_KEY=fake models --timeout nope → exit 2, code=GEV_INPUT_INVALID
```

`NewModelsCmd` rejects bad `--base-url` via `url.Parse` (must be absolute), bad `--timeout` via `time.ParseDuration`, and missing key before constructing the client.

### Go-level suite

```
$ go test ./internal/... ./cmd/...  → all packages ok
  PASS TestHomeIsOfflineAndReportsOnlyCredentialReadiness
  PASS TestVersionUsesInjectedBuildValuesAndModelsUseAuth
  PASS TestModelsMissingCredentialDoesNotCreateClient
  PASS TestValidateConflictsAndFailuresAreUsageErrors
  PASS TestValidateNeverCreatesClientAndDoesNotExposeState
  PASS TestVersionCommandPrintsSingleJSONLine
$ nix develop -c make check → true
```

### Paid production — `gev models` and default-output `gev ask`

Both runs through the real binary; synthetic state; `jev-1.13.0` pin; key from env inside the process; leak-scan over the raw captures clean. Raws under `.tmp/d3/raw/`.

| Run | Latency | Exit | Stderr | Stdout shape | Key numbers |
| --- | --- | --- | --- | --- | --- |
| `gev models` (real) | 626 ms | 0 | empty | one parseable JSON line, 2 aliases | `models: [jev-latest, jev-preview]` |
| `gev ask --questions q.json --state-file - --model jev-1.13.0` (default output) | 663 ms | 0 | empty | one parseable JSON line, **no `--output` flag used** | resolved `jev-1.13.0`; 391 in / 73 out tokens; choice sum = 1.0; score sum = 1.0; confidence in [0,1] |

**Cost** (input only, $42/Btok; output free; `docs.typesafe.ai/models`):
- `models` call — `GET /v1/models` is not billed by the documented pricing model (no `usage` field returned); the live API confirmed the call itself costs $0.00 against the request-budget price list.
- `ask` — 391 × $42 / 1e9 ≈ **$0.0000164**.

**Default-output proof via fake server**: same single-mode fake on two ports; one probe without `--output`, the other with `--output json`. **Byte-identical** stdout (`diff -q` reports no difference). The default emits JSON because there is no other renderer to choose.

### Cross-references

- D2-18/19 — closed in docs/qa/d2-ask-verification.md.
- F-D2-2 / F-D2-3 — closed by TASK-0011.
- TASK-0019 — **not closed** per instructions; final paid release verification must repeat at the exact release commit after TASK-0014.

### Documentation observation (no defect)

`--state-json` accepts a file path (or `-` for piped stdin), not an inline JSON literal — consistent with the `ask` command. A first-time operator may try `--state-json '{"k":"v"}'` and the source reader treats the curly-brace string as a path, returning `GEV_INPUT_INVALID: source … does not exist`. Worth a one-line example in `gev validate --help` if/when help text is upgraded (current help is Cobra-generated list form). Not a code defect; no fix proposed.
