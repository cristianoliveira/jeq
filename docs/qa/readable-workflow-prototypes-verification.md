# Readable workflow prototypes verification (TASK-0022)

Status: independent QA verification of the Python readable-workflow prototypes as **temporary executable specifications** for TASK-0023/0024 — not the final daily UX. No code, plans, or dependencies changed during this run; nothing committed.
Commit under test: **093d2e6713b5c1a86ed3b864a4a1314d5fd95a9f** ("fix: normalize multi-level Score evidence (TASK-0022)"). Working tree clean.
Runtime / dependency diff vs the v1 release candidate **96bab2c**: empty (TASK-0019 paid evidence remains valid). TASK-0022 changes live entirely under `examples/python/` plus `flake.nix` (adds Python) and three plan docs in `plans/`. The Python subprocess adapter (`examples/python/gev_cli.py`) is **explicitly temporary** — `plans/todo/0024-workflow-cli-and-manifest-based-examples.md` is the card that removes it and replaces the prototype workflows with native `gev` primitives.

## Result summary

| Check | Result |
| --- | --- |
| `make check` (Funzzy pipeline) exits 0 with `true` | **PASS** |
| Full `examples/` suite green (7 top-level cases, all Python + Bash tests) | **PASS** |
| Full repo `go test ./...` green | **PASS** |
| Support routing is one-stage (single `gev ask`, no cascade) | **PASS** |
| Release readiness zero-stage local blocker (no `gev` calls) | **PASS** |
| Release readiness one-stage semantic branches (`pass` / `canary` / `review` / `block` / `uncertain`) | **PASS** |
| Raw Score normalization 0..3 → policy scale 0..1 | **PASS** |
| Incident one-stage low-confidence stop (no second `gev` call) | **PASS** |
| Incident two-stage happy path (two `gev` calls, correlated decision) | **PASS** |
| Two-stage usage aggregation (input/output tokens summed) | **PASS** |
| Allowlisting enforced for rollout choices and per-category runbooks | **PASS** |
| Operational status 1 / 2 / 130 forwarded with GEV structured docs | **PASS** |
| No raw input leak in any receipt (`assertNoRawInput`) | **PASS** |
| No `TYPESAFE_API_KEY` access or echo in any Python script | **PASS** |
| `gev_cli.py` subprocess adapter marked temporary (per `plans/todo/0024…`) | **PASS** |

**15 pass · 0 fail · 0 deferred · 0 blocking findings · 0 contract holes**

## Evidence

### Hard gate

```
$ git status --short                     → (clean)
$ git diff 57a0166..HEAD -- cmd internal go.mod go.sum
                                       → (empty; runtime/dependencies unchanged)
$ nix develop -c make check             → true   (exit 0)
$ go test ./examples/... -count=1       → ok (7 top-level cases, including 4 Python ones)
$ go test ./... -count=1                → all packages ok
```

The four Python top-level tests verified in isolation:
- `TestPythonSupportRouterReceiptAndConfidencePolicy` — allowlisted/low-confidence branches
- `TestPythonReleaseReadinessBlockPolicyAndUncertainty` — zero-stage blocker + one-stage policy + raw-Score normalization + low-confidence → uncertain
- `TestPythonIncidentCascadeAndCandidateValidation` — two-stage happy path, low-confidence stops before second call, invalid runbook rejected
- `TestPythonOperationalStatusPropagation` — GEV exits 1 / 2 / 130 forwarded unchanged with structured docs

### Per-checkpoint evidence

**Support routing is one-stage.** `examples/python/support_router.py` is documented as "Readable one-stage support routing workflow." Test asserts `stage_count == 1`; exactly one `gev` request per ticket; `usage` and `answer.confidence` preserved; low confidence → `human_review`; unknown route → `human_review`.

**Release readiness zero-stage local blocker.** `local_blockers()` rejects `tests_passed == False` and `known_vulnerabilities` non-empty inputs BEFORE any `gev` call. The test asserts `stage_count == 0` and `api.count() == 0` (zero requests). Exit 10 with `decision="block"` and a structured reason — distinct from the semantic block path so deterministic blockers don't consume spend.

**Release readiness one-stage semantic branches.** Single `gev ask` with three questions (risk score, manual review noul, rollout choice). Policy: `risk >= 0.80 || manual >= 0.80 || rollout == "hold"` → block (exit 10); `risk <= 0.30 && manual <= 0.30 && rollout == "full"` → pass (exit 0); `risk <= 0.60 && manual <= 0.60 && rollout == "canary"` → canary (exit 0); otherwise review (exit 10). Low confidence or invalid runbook choice → uncertain (exit 11) via `InputFailure`. All branches asserted in `TestPythonReleaseReadinessBlockPolicyAndUncertainty`.

**Raw Score normalization 0..3 → 0..1.** `examples/python/release_readiness.py::normalize_score` returns `(raw, round(raw / MAX_SCORE_LEVEL, 6))`. The rounding is essential: the previous-gen failure (`raw_high_risk_score_blocks_after_normalization`) was caused by `2.4 / 3 == 0.7999999999999999 < 0.80` flipping the boundary; the fix rounds to 6 decimals so `2.4 / 3 → 0.8` and `0.2 / 3 → 0.066667`. Test asserts both trace values (`0.8` and `0.066667`) and the decision (`block` / `pass` respectively).

**Incident one-stage low-confidence stop.** `examples/python/incident_triage.py::stage_one` returns `None` when category confidence is below `MIN_CONFIDENCE`; the orchestrator short-circuits to `human_review` at `stage_count=1` without a second `gev` call. Test asserts `api.count() == 1` and `stage_count == 1`.

**Incident two-stage happy path.** Stage 1 asks `category`; if confident, stage 2 asks `severity`; runbook is selected from the category-scoped allowlist. The test asserts `stage_count == 2`, `decision.action == "runbook_selected"`, `decision.runbook_id` matches the chosen candidate, and `api.count() == 2`.

**Two-stage usage aggregation.** `gev_cli.add_usage` sums numeric fields and preserves any non-numeric ones. The two-stage happy-path test asserts `usage.input_tokens == 20` and `usage.output_tokens == 4` — the sum of two fake stages (10+10, 2+2). The low-confidence stop test asserts the single-stage usage (10, 2) is preserved unchanged.

**Allowlisting.**
- Release: `rollout_choice not in {"full", "canary", "hold"}` → `InputFailure` → uncertain receipt, exit 11.
- Incident: `runbook_id not in allowed` (where `allowed` is the category's runbook list) → `InputFailure` → uncertain receipt, exit 11.
- Routing: chosen option outside `{"billing","technical","security"}` would fail in the same way.
The `TestPythonIncidentCascadeAndCandidateValidation/invalid_selected_runbook_is_rejected` subtest pins the runbook allowlist at the file boundary.

**Operational status 1 / 2 / 130 forwarded.** `TestPythonOperationalStatusPropagation` runs three subtests:
- `auth status one`: fake server returns 401 → python script exits 1 with `code=GEV_AUTH_REJECTED`.
- `usage status two`: wrapper shim invokes `gev --output toon` → python script exits 2 with `code=GEV_INPUT_INVALID`.
- `interrupt status 130`: wrapper shim exits 130 → python script exits 130 with `code=GEV_INTERRUPTED`.
This mirrors the Bash-suite `TestChangeRiskGatePolicyAndOperationalStatus` pattern — the wrapper-shim approach is the correct design for a subprocess-layer test (real `SIGINT` against a child shell is racy).

**No raw input leak.** `assertNoRawInput` is called on every receipt with a representative input fragment: the ticket text for support routing, `"blocked-release"` for the release local blocker, `"ready-release"` for the release semantic pass, `"INC-PY-1"` for the incident inputs. All assertions pass.

**No `TYPESAFE_API_KEY` access in any Python script.** `rg "os\.environ" examples/python/` shows Python scripts read only `GEV_BIN`, `GEV_BASE_URL`, `GEV_MODEL`. The credential is consumed by the compiled `gev` binary inside the subprocess call; the Python workflow code never sees it. The Go test passes `TYPESAFE_API_KEY=examples-test-key` (a literal fake value, not a real key) via the subprocess `Env`. Leak scan: no key material, no real prefix, no `Bearer` string in any Python source or fixture.

### `gev_cli.py` is the temporary subprocess adapter — confirmed

The `gev_cli.py` module docstring explicitly says "Small subprocess adapter for the compiled gev CLI." It owns transport and process plumbing only; workflow modules (support_router, release_readiness, incident_triage) keep judgment policy in named Python functions. `plans/todo/0024-workflow-cli-and-manifest-based-examples.md` is the explicit owner for removing it. This prototype pattern is correct for an executable specification: the workflow semantics are stable Python, while the binding to `gev` is a small replaceable adapter.

## Readability assessment (brief)

Reading the three workflow files top-to-bottom:
- `support_router.py` (~70 lines): input → ask → confidence-floor + allowlist → receipt. One screen.
- `release_readiness.py` (~140 lines): local_blockers → ask → normalize + bounded + confidence-floor → policy ladder → receipt. Two screens, with policy thresholds named at the top.
- `incident_triage.py` (~170 lines): stage_one → (gate) → stage_two → allowlist → aggregate usage → receipt. Three screens; the two-stage shape is obvious from the function names.

These are reliable **behavioral oracles** for TASK-0023 (engine) and TASK-0024 (manifest): each file reads as a specification that a native `gev` primitive can replace without changing semantics. Threshold numbers (`RISK_PASS_MAX`, `RISK_CANARY_MAX`, `RISK_BLOCK_MIN`, `MIN_CONFIDENCE`, `MAX_SCORE_LEVEL`) are exposed at module top, ready to become CLI flags or manifest fields.

## Findings

**No findings.** Two non-blocking observations carried forward from prior waves:

- `jq` is in `flake.nix` from TASK-0021; the v1 release candidate `96bab2c` did not include it. The Python workflows don't use `jq` (they shell out to `gev` and parse its JSON directly), so this only matters for the Bash examples.
- The CLI does not validate `--base-url` as an absolute URL for `gev ask` (it does for `gev models`). The Python examples use absolute URLs everywhere, so the asymmetry is invisible here; recorded for any future shell-side unification.
- The 130 propagation test uses a wrapper shim rather than a real `SIGINT`. Same design choice as the Bash examples; right call for subprocess-layer tests.

## Cross-references

- TASK-0019 paid evidence (docs/qa/live-gev-verification.md) remains valid; the runtime/dependency diff from the v1 release candidate is empty.
- TASK-0021 verification (docs/qa/unix-workflow-examples-verification.md) — the Bash examples stay green; this run re-verifies they coexist with the Python prototypes in the same suite.
- TASK-0023 (versioned decision workflow engine) and TASK-0024 (workflow CLI and manifest-based examples) are the next cards; TASK-0022 is the executable specification they replace.
