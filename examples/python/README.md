# Readable Python workflows

Start here when the decision story matters more than shell mechanics. These
programs use only the Python standard library and invoke the compiled `gev`
binary; they do not contain an HTTP client or a workflow DSL.

## Shape

```text
stdin JSON/text
    |
    v
named deterministic policy ---> gev ask ---> typed answers
    |                                      |
    +---------- JSON receipt  <------------+
```

`gev_cli.py` is transport plumbing only. Each workflow keeps thresholds,
allowlists, stage transitions, and exit policy in readable named functions.
Model output is data: no runbook or action is ever executed.

## Workflows and call budgets

| Program | Input | Calls | Policy exits |
| --- | --- | ---: | --- |
| `support_router.py` | One ticket text | 1 | `0` (including `human_review`) |
| `release_readiness.py` | Release evidence JSON | 0 or 1 | `0` pass/canary, `10` block/review, `11` uncertain |
| `incident_triage.py` | Incident JSON | 1 or 2 | `0` selected, `11` human review |

Focused guides: [support router](docs/support-router.md), [release readiness](docs/release-readiness.md), and [incident triage](docs/incident-triage.md).

Release hard blockers (`tests_passed: false` or non-empty
`known_vulnerabilities`) make zero paid calls and exit `10`. Incident triage
makes the second call only after confident stage-one evidence. Otherwise every
call consumes the account's model budget; live usage is opt-in and not part of
CI. Keep `TYPESAFE_API_KEY` in the environment, never in state, argv, or
receipts. Set `GEV_BIN`, `GEV_BASE_URL`, and `GEV_MODEL` for local or approved
runs.

## Policy tables

### Release readiness

| Rule | Decision | Exit |
| --- | --- | ---: |
| Local blocker | block | 10 |
| risk `<= 0.30`, manual review `<= 0.30`, full rollout | pass | 0 |
| risk `<= 0.60`, manual review `<= 0.60`, canary rollout | canary | 0 |
| risk `>= 0.80`, manual review `>= 0.80`, or hold rollout | block | 10 |
| Low confidence or unsupported evidence | uncertain | 11 |

Thresholds are demonstrations, not calibrated release controls.

### Incident cascade

```text
stage 1: category + severity + page
       low confidence? ---- yes ---> human_review (11)
              |
              no
              v
load versioned category runbooks
              |
stage 2: choice criteria = only candidate IDs
              |
       valid ID + confidence? ---> runbook_selected (0)
              |
              no
              v
         human_review (11)
```

The selected runbook is reported, never invoked. A receipt keeps typed answer
evidence, model, usage, and stage count but never copies raw incident or ticket
state. Operational gev failures retain gev's structured stdout and exit `1`,
`2`, or `130`; stderr remains diagnostics.

## Commands

```sh
python3 examples/python/support_router.py < examples/support-routing/fixtures/ticket.txt
python3 examples/python/release_readiness.py < examples/python/fixtures/release.json
python3 examples/python/incident_triage.py < examples/python/fixtures/incident.json
```

The adjacent JSON specs and fixtures are versioned examples. They are evidence
for future `gev gate` and `gev map` primitives, not a proposal for a Python
workflow framework.
