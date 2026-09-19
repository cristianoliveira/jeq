# TASK-0025 reduce and user-config verification

Independent verification of TASK-0025 commits:

- `78ad492` — add one-call reduce and user model config
- `f69cd65` — harden reduce invariants and config precedence

The worktree was clean before verification. This report is the only file changed by this verification; no code or plans were edited.

## Verdict

**PASS.** No blocking findings. The one-call aggregate, shared question sources, model configuration, operational error classes, and discovery contracts are implemented and verified.

One low-severity design observation is recorded at the end: map and reduce duplicate some CLI orchestration policy. It is source-backed but does not change current behavior or acceptance.

## Counts and gates

- Funzzy gen 204: full gate PASS — `nixfmt --check flake.nix && nix flake check && fzz check && go build ./... && golangci-lint run && go test ./...`.
- Coverage command under the Nix toolchain: `go test ./... -coverprofile=/tmp/gev-task0025.cover.nix`; exit 0, total **79.9%**, above the **79.4%** floor.
- Focused TASK-0025 tests: all selected config, source, reduce, pipeline, and optional-source tests passed.
- Architecture/import tests: PASS.
- `nix develop -c govulncheck ./...`: **No vulnerabilities found**.
- Compiled binary: `nix develop -c go build -o /tmp/gev-task0025 ./cmd/gev` passed.

## Reduce acceptance

A deterministic local fake API recorded requests and returned a complete typed response with an unknown `server_extra` field.

| Scenario | Result |
| --- | --- |
| JSON array reduce | PASS; output is one `{items,_gev}` JSON envelope |
| NDJSON reduce | PASS; values collected in input order into one array |
| Exact collection request state | PASS; recorded state was exactly `[{"id":1},{"id":2}]` |
| Exactly one API call | PASS; one call per reduce invocation for both JSON and NDJSON |
| Complete response extras | PASS; `_gev.batch.server_extra.kept` survived |
| Map → reduce → gate | PASS; 2 map calls + 1 reduce call, then offline gate; final policy was `pass` |
| JSON output | PASS; output parses as one deterministic JSON document |

The chained output preserved each mapped item under `items`, appended the aggregate receipt under `_gev.aggregate`, and appended the gate receipt under `_gev.release_policy`.

## Reduce local validation and bounds

`TestReduceCollectionBoundsRejectEmptyAndExcessItems` and focused binary/CLI tests cover:

- empty JSON collection;
- non-array JSON input;
- malformed JSON / malformed NDJSON item;
- recursive duplicate keys;
- invalid framing;
- per-item and collection byte limits;
- maximum item count and the 10,001st NDJSON item;
- oversized inline questions;
- invalid question-source combinations.

These failures occur before client construction or evaluation. The shared source-conflict test asserts zero dependency reads for both `map` and `reduce`. Invalid reduce config also asserts zero client calls. Errors are structured with stable codes and recovery text through the command runner.

## Shared question sources

`map` and `reduce` expose the same strict question-source interface:

- exactly one of `--questions` (file) or `--questions-json` (inline document);
- the same `contract.DecodeQuestionsDoc` schema and local `ValidateRequest` probe;
- the same size bound (`MapMaxRecordBytes`);
- stdin remains the record/input stream, so `--questions -` is rejected as a source conflict;
- both commands advertise the source flags in help;
- both reject source conflicts before reading stdin, reading files, environment, or creating a client.

`map` additionally supports its separate native `--request-pointer` mode; it rejects mixing that mode with question sources, model, state pointer, or config.

## Model configuration

The compiled binary and focused tests verify precedence:

```text
--model > TYPESAFE_DEFAULT_MODEL > ${XDG_CONFIG_HOME:-$HOME/.config}/gev/config.json > jev-latest
```

Observed request models were `config-model`, `env-model`, and `flag-model` in the three corresponding runs. A missing default config was ignored and fallback resolved to `jev-latest`; an explicit missing config returned exit 2. Home discovery reports both `default_model` and `default_model_source`.

Config behavior is strict and bounded:

- only `default_model` is accepted;
- malformed JSON, wrong type, empty value, duplicate key, unsupported field, and oversized document fail;
- explicit `--config` is required to exist;
- default user config uses `XDG_CONFIG_HOME` first, then `$HOME/.config`, and missing default config is ignored;
- credentials and base URL are not accepted as config fields;
- flag and environment precedence short-circuits lower sources (tested with panic readers/environment callbacks).

## Help, discovery, and privacy

`gev reduce --help` exposes `--questions`, `--questions-json`, `--model`, `--config`, framing, limits, timeout, and retry flags. `gev map --help` exposes the same shared question flags plus its state/native-request options.

The structured home view includes `map`, `reduce`, `gate`, `ask`, `models`, `validate`, `version`, and `help` when dependencies are available. With unavailable dependencies (`AskDeps{}`), it exits successfully and advertises only `version` and `help`. It remains offline.

ADR 0005 explicitly defines reduce as one aggregate request, not an iterative fold. The release example documents the distinction: one complete collection is sent, there is no hidden per-item request, and the aggregate envelope retains `items` plus `_gev.batch_risk`.

The examples warn that map/reduce preserve input state and show `jq` projections before logs or sharing. The compiled reduce output therefore keeps privacy caller-managed as documented; it does not print credentials or create a secret-bearing diagnostic.

## Operational behavior

Against a deterministic local status/delay server:

| Condition | Result |
| --- | --- |
| HTTP 401 | exit 1, `GEV_AUTH_REJECTED`, credential detail redacted |
| request timeout | exit 1, `GEV_TIMEOUT` |
| SIGINT during request | exit 130, `GEV_INTERRUPTED` |
| local malformed/invalid input | exit 2, `GEV_INPUT_INVALID` |
| aggregate policy gate reject/uncertain | gate retains its documented 10/11 policy statuses |

## Findings

### Blocking findings

None.

### Observation O1 — duplicated orchestration policy

The dogfood advisory identified possible `duplicated_policy` with probability 0.65 and confidence 0.59. Source inspection confirms materially overlapping orchestration in `internal/cli/map.go` (`runMap`) and `internal/cli/reduce.go` (`runReduce`): output validation, flag/source checks, retry and timeout parsing, dependency availability, stdin buffering, question loading, model resolution, credential lookup, base URL resolution, client construction, and diagnostic wiring. The aggregate-specific collection parsing and map-specific per-record processing remain different, so this is not a correctness issue.

A future refactor could centralize only the stable boundary policy while keeping map/reduce semantics separate. No refactor is recommended as part of TASK-0025 verification; the current duplication is explicit and covered by tests.

## Final status

- Verdict: **PASS**.
- Blocking findings: **0**.
- Coverage: **79.9%**, floor **79.4%** met.
- Code/plans/dependencies changed by verification: **0**.
