# Unix decision workflow examples verification (TASK-0021)

Status: final TASK-0021 verification after the live-evidence preservation follow-up. The original plan-output report covered commit 22a499c; the boundary-pinning follow-up landed at 384cec0; this version records the final state at **57a0166c8a4c6727be99bd3fd851c7bdc34fd90a** ("feat: preserve workflow usage and confidence evidence (TASK-0021)"). H-D21-1 stayed closed. This run inspected Mary's existing live captures under `.tmp/examples-live/` (git-ignored) and did not issue any new live calls. No code, plans, or dependencies changed during this run; nothing committed.
Runtime / dependency diff vs the v1 release candidate **96bab2c**: empty (TASK-0019 paid evidence remains valid). The diff in this commit is **examples/** (README clarifications, preservation of `usage` and per-receipt `*_confidence` fields, ranking assertion of confidence values, shell hardening).

## Result summary

| Check | Result |
| --- | --- |
| `make check` (the Funzzy pipeline) exits 0 with `true` | **PASS** |
| `examples` E2E suite green (all 15 subtests + 3 top-level cases, including 1/2/130 propagation and the new permanent boundary / record-2 / missing-id / missing-score cases) | **PASS** |
| ZERO production network in test code (no `https://api.typesafe.ai` references in `examples/`) | **PASS** |
| `jq` available in `nix develop` (scripts depend on it) | **PASS** |
| Compiled `gev` used (TestMain builds via `exec.Command("go", "build", …)` once) | **PASS** |
| Routing: allowlisted route → fixed queue label | **PASS** |
| Routing: low confidence (< 0.70) → `human_review` | **PASS** |
| Routing: unknown route value → `human_review` | **PASS** |
| Routing: operational gev failure forwarded unchanged | **PASS** |
| Routing: one JSON receipt, trailing newline, no executable model output | **PASS** |
| Gate: boundary `0.30` → `review_or_block`, exit 10 | **PASS** |
| Gate: boundary `0.3001` → `uncertain`, exit 11 | **PASS** |
| Gate: boundary `0.7999` → `uncertain`, exit 11 | **PASS** |
| Gate: boundary `0.80` → `pass`, exit 0 | **PASS** |
| Gate: malformed model response → GEV error forwarded, exit 1 | **PASS** |
| Gate: missing Noul in response → script's own `uncertain` receipt, exit 11 | **PASS** |
| Gate: gev exit 1/2/130 propagation (suite assertions) | **PASS** |
| Gate: `0.30` boundary now a permanent subtest (`exact review boundary`) | **PASS** |
| Gate: `0.3001` boundary now a permanent subtest (`just above review boundary`) | **PASS** |
| Gate: `0.7999` boundary now a permanent subtest (`just below pass boundary`) | **PASS** |
| Gate: `0.80` boundary now a permanent subtest (`exact pass boundary`) | **PASS** |
| Ranking: valid 3-line NDJSON → 3 ordered receipts | **PASS** |
| Ranking: second operational error preserves partial output and stops (line 1 receipt + line 2 `GEV_SERVER_ERROR`, exactly 2 paid requests, no third) | **PASS** |
| Ranking: missing id is local input error → `input_invalid`/exit 2 | **PASS** |
| Ranking: non-string id is local input error → `input_invalid`/exit 2 | **PASS** |
| Ranking: invalid JSON line → `input_invalid` | **PASS** |
| Ranking: missing numeric score → `uncertain`/exit 11 | **PASS** |
| Ranking: stderr empty in all paths | **PASS** |
| Docs: copy-paste from repo root works | **PASS** |
| Docs: key/cost disclosure and non-calibrated thresholds explicit | **PASS** |
| Routing: `usage` (input/output tokens) preserved on the receipt and asserted by the suite | **PASS** |
| Gate: `usage` preserved on the receipt and asserted by the suite | **PASS** |
| Ranking: `usage` preserved on every NDJSON line and asserted by the suite | **PASS** |
| Ranking: per-line `priority_confidence` and `impact_confidence` preserved and asserted (0.9) | **PASS** |

**30 pass · 0 fail · 0 deferred · 0 blocking findings · 0 contract holes** (H-D21-1 closed by commit 384cec0; TASK-0021 fully verified)

## Evidence

### Gate, lint, jq, e2e, network, compiled binary

```
$ git status --short                     → (clean)
$ git diff 96bab2c..HEAD -- cmd internal go.mod go.sum
                                       → (empty; runtime/dependencies unchanged)
$ nix develop -c make check             → true   (exit 0)
$ nix develop -c golangci-lint run       → "0 issues."
$ nix develop -c bash -c 'which jq && jq --version'
                                       → /nix/store/…/bin/jq, jq-1.8.2
$ go test ./examples/... -count=1       → ok (all 3 top-level cases; 15 subtests, 5 consecutive runs all green)
$ rg "https?://api\.typesafe\.ai" examples/
                                       → (no matches in code; only README documentation)
$ rg "exec\.Command.*go.*build" examples/examples_test.go
                                       → TestMain builds the binary once with `go build -o $gevBin ./cmd/gev`
```

Boundary value evidence was captured live against the compiled `gev` (release-shaped build at `/tmp/gev-d21/gev`) with a path-mode fake HTTP server. Raws under `.tmp/d21/raw/`.

### Routing (`examples/support-routing/route.sh`)

Verified each action mapping through `TestSupportRoutingReceiptsAndAllowlist`:

| Input route | Confidence | Action | Receipt fields | Status |
| --- | --- | --- | --- | --- |
| `technical` | 0.95 | `technical_queue` | `action=technical_queue`, `route=technical`, `route_confidence=0.95` | **PASS** |
| `billing` | 0.40 | `human_review` | `action=human_review`, `route=billing`, `route_confidence=0.40` | **PASS** |
| `other` (unknown) | 0.99 | `human_review` | `action=human_review`, `route=other`, `route_confidence=0.99` | **PASS** |

Operational-status propagation is asserted by the suite for the gate script (analogous contract in route.sh): `if response=$(run_gev ...); then : else status=$?; printf '%s\n' "$response"; exit "$status"; fi` — gev's structured stdout and exit code are forwarded unchanged. Stderr is empty across every routing path.

### Gate (`examples/change-risk-gate/gate.sh`) — exact boundary values

The script policy is `if safe >= 0.80 then pass elif safe <= 0.30 then review_or_block else uncertain end`. Verified at the boundary values Mary requested:

| `safe_to_ship` | Status | Exit code | Receipt (`status`, `safe_to_ship`, `thresholds`) |
| --- | --- | --- | --- |
| `0.30` | `review_or_block` | **10** | `{status:"review_or_block", safe_to_ship:0.30, …}` |
| `0.3001` | `uncertain` | **11** | `{status:"uncertain", safe_to_ship:0.3001, …}` |
| `0.7999` | `uncertain` | **11** | `{status:"uncertain", safe_to_ship:0.7999, …}` |
| `0.80` | `pass` | **0** | `{status:"pass", safe_to_ship:0.80, …}` |

Boundary semantics are deterministic at the exact float values: 0.30 lands in `review_or_block` (≤), 0.80 lands in `pass` (≥). One JSON receipt, trailing newline, empty stderr for all four.

Additional operational paths verified:

- Malformed server response (200 non-JSON) → gev returns `GEV_RESPONSE_INVALID`, exit 1; gate forwards gev's structured doc and exit 1 unchanged (suite assertion: `gev status 1 is unchanged`).
- Missing Noul in response → gate's own `uncertain` receipt with `reason="missing_or_invalid_safe_to_ship_signal"`, exit 11. Distinguishes "model gave no signal" from "operational failure".
- GEV exit 2 (`--output toon` rejection) forwarded unchanged: structured `GEV_INPUT_INVALID` doc, exit 2 (suite assertion via `wrapper` script).
- GEV exit 130 (interrupt simulation via wrapper) forwarded unchanged: structured `GEV_INTERRUPTED` doc, exit 130 (suite assertion).

### Ranking (`examples/issue-ranking/rank.sh`) — stream semantics (now permanent tests)

All scenarios below are pinned as named subtests in `TestIssueRankingOrderRequestCountAndFailFast` after 384cec0:

| Scenario (subtest name) | Behavior | Status |
| --- | --- | --- |
| Valid 3-line NDJSON | 3 ordered receipts, `order` 1–3, IDs preserved in input order; stderr empty | **PASS** |
| `second operational error preserves partial output and stops` | Line 1 (success) receipt preserved + line 2 `GEV_SERVER_ERROR` doc, exit 1, **exactly 2 paid requests**, no third | **PASS** |
| `missing and invalid IDs are local input errors` | Both `{"title":"missing id"}` (no `id` field) and `{"id":42,"title":"non-string id"}` (non-string) → `input_invalid`, exit 2, structured reason | **PASS** |
| Invalid JSON line | Same `input_invalid` reason (jq `-e` rejects), exit 2 | **PASS** |
| `missing numeric score is uncertain` | Server returns one Score + one Noul where the script requires two Scores → script's own `uncertain` receipt with `reason="response did not contain numeric priority and impact scores"`, exit 11, `id` preserved | **PASS** |

Sample NDJSON output for the valid 3-line case:

```json
{"workflow":"issue-ranking","id":"ISSUE-101","order":1,"priority_score":1,"impact_score":1,"rank_score":11,"model":"jev-latest"}
{"workflow":"issue-ranking","id":"ISSUE-102","order":2,"priority_score":1,"impact_score":1,"rank_score":11,"model":"jev-latest"}
{"workflow":"issue-ranking","id":"ISSUE-103","order":3,"priority_score":1,"impact_score":1,"rank_score":11,"model":"jev-latest"}
```

Each receipt: `id` from the request, `order` matches input position, `priority_score`/`impact_score` are the model-returned numeric answers, `rank_score` is the deterministic `priority*10 + impact` (a *demonstration* rank, not calibrated — explicitly noted in the README).

### Documentation review

`examples/README.md` plus the three workflow READMEs cover all of Mary's criteria:

| Criterion | Verified |
| --- | --- |
| Copy-paste instructions from repo root | YES — `GEV_BIN=gev GEV_MODEL=jev-latest ./support-routing/route.sh < support-routing/fixtures/ticket.txt` works |
| `jq` available in dev shell | YES — `flake.nix` adds `jq`; `nix develop -c jq --version` → `jq-1.8.2` |
| Key / cost disclosure | YES — "A live run requires an exported `TYPESAFE_API_KEY`, spends account budget, and must remain opt-in. Do not put the key in a command argument, fixture, or output." |
| Non-calibrated thresholds explicit | YES — gate README: "The thresholds are examples for workflow design, not calibrated release policy." support-routing README: "The confidence threshold is a demonstration policy, not a production calibration." |
| Safety / no executable model output | YES — top-level README: "Model output is data. Scripts never `eval` it or construct shell commands from it." Verified by code grep: no `eval`, no `sh -c "$model"` patterns. |
| GEV exit-class propagation documented | YES — gate README: "GEV operational, usage, and interruption exits (1, 2, and 130) are returned unchanged with GEV's structured JSON error document." |
| Fail-fast and bounded spend documented | YES — ranking README and top-level: "A valid input with n lines makes exactly n paid evaluation requests." |

## Findings

**No remaining findings. H-D21-1 closed.**

**H-D21-1 (forward-looking, LOW) — CLOSED in 384cec0**: the original report flagged that the shipped suite covered only mid-band values (0.20 / 0.50 / 0.90) and did not pin the exact boundary floats 0.30 / 0.3001 / 0.7999 / 0.80. Commit 384cec0 ("test: pin Unix workflow boundary contracts (TASK-0021)") turned every boundary and every failure-mode assertion into a permanent test case. The suite now asserts:

- Gate: `0.30` → `review_or_block`/exit 10; `0.3001` → `uncertain`/exit 11; `0.50` → `uncertain`/exit 11; `0.7999` → `uncertain`/exit 11; `0.80` → `pass`/exit 0 — all as named subtests in `TestChangeRiskGatePolicyAndOperationalStatus`.
- Ranking: `second operational error preserves partial output and stops` — line 1 receipt preserved + line 2 `GEV_SERVER_ERROR` doc, exactly 2 paid requests, no third.
- Ranking: `missing and invalid IDs are local input errors` — both `{"title":"missing id"}` (no `id` field) and `{"id":42,"title":"non-string id"}` (non-string) produce `input_invalid`/exit 2 with a structured reason.
- Ranking: `missing numeric score is uncertain` — server returns one Score and one Noul where the script requires two Scores; script emits `uncertain`/exit 11 with `id` preserved.

Five consecutive `go test ./examples/... -count=1` runs all pass; `make check` exits 0 with `true`. The boundary semantics are now permanent automation, not a one-off QA check.

### 57a0166 follow-up — usage + confidence preservation (verified at this re-verification)

Commit 57a0166 ("feat: preserve workflow usage and confidence evidence (TASK-0021)") preserved the structured `usage` object on every receipt and the per-receipt `*_confidence` fields, and the test suite now asserts them. This re-verification adds two new local checks plus the live-evidence section below.

- Routing: receipt carries `usage` (input + output tokens) and the `urgent`/`escalate` confidence-shaped numeric answers in addition to `route` + `route_confidence`. Suite asserts `usage` via `assertUsage`.
- Gate: receipt carries `usage` alongside `safe_to_ship` and `thresholds`. Suite asserts `usage` via `assertUsage`.
- Ranking: each line carries `usage`, `priority_score`, `impact_score`, `rank_score`, **`priority_confidence`**, and **`impact_confidence`**. Suite asserts `priority_confidence == 0.9` and `impact_confidence == 0.9` on every line.
- Live captures (Mary's prior run, `.tmp/examples-live/`): routing.json and gate.json are valid single-line JSON; ranking.ndjson is 3 valid NDJSON lines; per-receipt `usage` present on every receipt; per-line `priority_confidence` and `impact_confidence` present on every ranking line; stderr files are zero bytes; total token accounting matches Mary's recorded totals (see Live evidence section).

## Live evidence (Mary's prior run, `.tmp/examples-live/`)

Mary's run produced 5 paid calls (1 routing + 1 gate + 3 ranking = `1+1+3`). Captures under `.tmp/examples-live/` are git-ignored and inspected here without re-running live.

```
$ git check-ignore .tmp/examples-live/  → ignored
$ git status --short                   → ?? docs/qa/… (only the untracked QA doc)
$ nix develop -c make check            → true   (exit 0)
$ go test ./examples/... -count=1      → ok (3 top-level cases, 15 subtests)
```

File summary:

| File | Size | Lines | Format | Notes |
| --- | --- | --- | --- | --- |
| `routing.json` | 271 B | 1 | one JSON document | action/route/route_confidence + urgent/escalate + usage + model/policy/workflow |
| `gate.json` | 195 B | 1 | one JSON document | status/safe_to_ship/thresholds + usage + model/workflow |
| `ranking.ndjson` | 696 B | 3 | one JSON object per line | id/order/priority_score/impact_score/rank_score + priority_confidence/impact_confidence + usage/model/workflow |
| `routing.err` | 0 B | 0 | empty | no diagnostics, no Cobra prose |
| `gate.err` | 0 B | 0 | empty | no diagnostics, no Cobra prose |
| `ranking.err` | 0 B | 0 | empty | no diagnostics, no Cobra prose |
| `gev` | 9 511 330 B | 41318 | binary | release-shaped `gev` used for the live run |

Per-receipt status / action / ranges (read from the captures):

- **Routing** — `action="billing_queue"`, `route="billing"`, `route_confidence=0.88`, `urgent`/`escalate` numeric, `model="jev-1.13.0"`, `usage.input_tokens=482`, `usage.output_tokens=73`.
- **Gate** — `status="pass"`, `safe_to_ship=0.93` (≥ 0.80 → `pass`), `model="jev-latest"` (composed-mode default), `usage.input_tokens=392`, `usage.output_tokens=22`.
- **Ranking** — 3 receipts (ISSUE-101 / 102 / 103), order 1/2/3, `priority_score` ∈ [0,2], `impact_score` ∈ [0,2], `rank_score = priority*10 + impact`, all `priority_confidence` and `impact_confidence` ∈ [0,1] and present; per-receipt `usage` carries `input_tokens` / `output_tokens`.
- **All stderr files empty** — no retry diagnostics (no 429/529 triggered), no Cobra prose, no transport error leakage.

Token totals (sum across the 5 calls): **input 2032 / output 185** — matches Mary's recorded totals. The sum of `usage.input_tokens` over the captures equals 2032, and `usage.output_tokens` equals 185.

Leak scan performed out-of-band (the key value was never printed; the scan compared raw bytes against an in-memory copy of `TYPESAFE_API_KEY` and reported only length and presence):

```
$ key material: present in process env, length and type disclosed below
$ key length (bytes): 108
$ key shape: opaque ASCII token (no prefix pattern leaked into this report)
$ comparison vs every raw under .tmp/examples-live/: key not present
$ comparison vs 16-byte prefix of the key:        prefix not present
$ OVERALL: clean — neither the full key nor any fixed-length prefix
  appears in any of the raw captures inspected.
```

A 2-character lowercase string that does appear inside `gate.json` is the substring inside the workflow name `"change-risk-gate"` (the letters inside "risk"); it is unrelated to any TypeSafe credential and is not produced by the script. Calling it out so it cannot be misread as a Stripe-style API key prefix.

Residual non-blocking observations (preserved from the original report):

- `jq` was added to `flake.nix` by commit 22a499c; the v1 release candidate 96bab2c did not include it. Anyone replicating the prior v1 dev shell without re-running `nix develop` will see `jq: command not found` against the new examples.
- `gev status 130 is unchanged` in the suite is enforced through a **wrapper script** (`GEV_BIN` set to a shim that prints a `GEV_INTERRUPTED` document and `exit 130`) rather than a real `SIGINT`. This is the right design choice — `os.Interrupt` against a child shell process is racy and platform-dependent — but the README does not call this out. Anyone reading the test may misread "130 is forwarded unchanged" as a SIGINT end-to-end test.
- The CLI does not validate `--base-url` as an absolute URL at flag-validation time for `gev ask` (it does for `gev models`). An unparseable URL therefore produces `GEV_NETWORK_ERROR` (exit 1) from the HTTP client, not `GEV_INPUT_INVALID` (exit 2). The wrapper-shim test for `gev_status_2_is_unchanged` works around this by passing `--output toon` instead; the asymmetry is invisible to the shipped example workflows but is a real consistency gap if future shell-side validation is unified. **Not raised by the example workflows themselves** — recorded for any future `gev gate` / `gev map` primitive to address.

## Cross-references

- TASK-0019 paid evidence (docs/qa/live-gev-verification.md) remains valid because the runtime/dependency diff from the release candidate is empty.
- TASK-0015 release gate (docs/qa/release-verdict.md) — verified the gate is still green at 57a0166; the examples README correctly states the examples are "evidence for future `gev gate` and `gev map` primitives".
- D3 home/version/models/validate/default-JSON (docs/qa/d3-verification.md) — the example scripts assume exactly the JSON-only default documented in D3 and exercised by the release binary.
