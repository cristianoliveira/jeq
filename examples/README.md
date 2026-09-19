# Unix decision workflow examples

These examples treat `gev` as a semantic Unix filter. Bash owns composition,
thresholds, exit policy, and side effects. `jq` owns JSON parsing and receipt
construction. `gev` owns typed judgment requests and structured operational
errors. No example adds a subcommand or workflow DSL.

## Workflows

| Directory | Input | Decision |
| --- | --- | --- |
| `support-routing` | One support ticket from stdin | Allowlisted queue or `human_review` |
| `change-risk-gate` | One diff from stdin | Pass, review/block, or uncertain |
| `issue-ranking` | Issue NDJSON from stdin | Correlated score NDJSON in input order |

Each directory contains a question document, representative fixture, executable
script, and focused README. Use `GEV_BIN` to select the binary, `GEV_MODEL` to
select the model, and `GEV_BASE_URL` to select an endpoint:

```sh
GEV_BIN=gev GEV_MODEL=jev-latest \
  ./support-routing/route.sh < support-routing/fixtures/ticket.txt
```

`GEV_BASE_URL` is optional. Without it, the normal TypeSafe endpoint is used.
Set it to a local `httptest` endpoint for development. A live run requires an
exported `TYPESAFE_API_KEY`, spends account budget, and must remain opt-in. Do
not put the key in a command argument, fixture, or output.

## Safety and policy

- Scripts use strict Bash mode and quoted argument arrays.
- Model output is data. Scripts never `eval` it or construct shell commands from
  it.
- Routing maps only three route values to fixed labels. Low confidence and
  unknown values go to `human_review`.
- Gate thresholds are explicit demonstrations, not calibrated release policy.
- Ranking uses fail-fast semantics so one valid line makes one request and a
  first failure stops further spend while preserving prior output.
- GEV exit `1`, `2`, and `130` remain operational statuses and their structured
  JSON documents are forwarded unchanged. Policy exits belong only to the gate
  (`0`, `10`, `11`).

The examples are evidence for future `gev gate` and `gev map` primitives. They
are not a proposal for a workflow language. Future primitives should preserve
explicit thresholds, allowlists, stream correlation, bounded spend, and clear
separation between model judgments and executable policy.
