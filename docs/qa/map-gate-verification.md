# TASK-0024 map/gate blocker retest

Retest of fix `0ba21126368a69eae0e2905a3c0715eb82868c4c` (`fix: harden map and gate CLI contract (TASK-0024)`). This retest was read-only except for this QA report; no code/plans/dependencies were changed and no commit was created.

## Verdict

**PASS — B1, B2, B3, B4, and O1 are resolved.** No blocking findings remain from the prior TASK-0024 review.

## Verification

- Pinned Nix jq: `jq-1.8.2`.
- Funzzy generation 125: full gate PASS (`nixfmt`, `nix flake check`, `fzz check`, build, lint, all Go tests).
- Focused regression tests passed:
  - `TestMapAndGateRejectUnsupportedOutputBeforeDependencies` (map and gate cases).
  - `TestHomeDiscoveryIncludesAvailableMapAndGate`.
- Compiled binary: `nix develop -c go build -o /tmp/jeq-task0024-fixed ./cmd/jeq` passed.

### B1 — incident jq compatibility

Executed the fixed incident pipeline with the pinned jq 1.8.2 and a deterministic local fake TypeSafe endpoint:

```text
category map -> jq --slurpfile catalog -> native request map -> gate
```

Result: **PASS**. Two API requests were made, the gate receipt was `pass`, and the final output contained the expected category, runbook, and policy evidence. The removed `--argfile` incompatibility is fixed.

### B2 — unknown catalog key

Ran the fixed jq transform with category `does-not-exist` under pinned jq 1.8.2.

Result: **PASS**. jq exited non-zero (`5`), emitted no output document, and reported:

```text
unknown catalog category: does-not-exist
```

Unknown catalog values are no longer silently filtered as an apparently successful empty stream.

### B3 — category instruction/path correctness

Asserted the final request and receipts:

```text
._jeq.category.answers.category.choice == "technical"
.request.questions.runbook.instructions ==
  "Select the approved runbook for category technical"
._jeq.runbook.answers.runbook.choice == "technical-api-outage"
._jeq.incident_policy.decision == "pass"
```

Result: **PASS**. The request now uses the actual category evidence path and does not produce `category unknown` for a recognized answer.

### B4 — unsupported output format

Ran the compiled binary with `--output text` for both commands, using unavailable/missing dependencies to prove validation ordering:

```text
jeq map  --output text ... -> exit 2, JEQ_INPUT_INVALID,
  unsupported output format "text"; reads/env/client calls: 0
jeq gate --output text ... -> exit 2, JEQ_INPUT_INVALID,
  unsupported output format "text"; reads/env/client calls: 0
```

Result: **PASS**. Both commands reject non-JSON output before input, environment, or client work.

### O1 — home discovery

Available dependencies:

```json
{"commands":["validate","models","gate","ask","map","version","help"]}
```

The compiled binary home view now includes both `map` and `gate`. The injected CLI test also passes.

Unavailable dependencies were verified with `cli.RunWithDeps(..., AskDeps{})`: exit 0, no stderr, and commands are restricted to `["version", "help"]`. No unavailable command is advertised.

## Final status

- B1 fixed and verified.
- B2 fixed and verified.
- B3 fixed and verified.
- B4 fixed and verified.
- O1 fixed and verified.
- Blocking findings: **0**.
- Code/plans/dependency changes by this verification: **0**.
- Commits created by this verification: **0**.
