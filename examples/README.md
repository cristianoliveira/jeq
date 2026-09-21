# Readable Unix pipelines

Start with `jeq examples` for installed, self-contained recipes. This repository
contains expanded executable references for those workflows. A rank result can be composed explicitly, for example: `jeq rank ... | jq '.items[:3] | map(.candidate)'`. Choice probabilities are relative; include an explicit fallback candidate when needed.

These examples use the composable primitives from ADR 0004. `jeq map` performs
one named semantic judgment per record; `jq` projects, joins, and constructs
native requests; `jeq rank` ranks candidates with Choice probabilities and `jeq gate` applies an explicit offline numeric policy.

The same JSON record remains the transport envelope. Before logging or sharing
it, project away sensitive state explicitly:

```sh
jq 'del(.customer_message, .details, ._jeq.route)'
```

A live run needs `TYPESAFE_API_KEY` and spends account budget. Use a local fake
endpoint for development. Model evidence is data: examples validate it as a catalog key or a numeric value, and never execute it. `rank`, `map`, and `reduce` resolve models as `--model`, `TYPESAFE_DEFAULT_MODEL`, user config (`${XDG_CONFIG_HOME:-$HOME/.config}/jeq/config.json`), then `jev-latest`; the config contains only `default_model`.


## Recommended pipelines

- [`smart-grep`](smart-grep/README.md): retrieve and group log excerpts locally,
  inspect a bounded shortlist, then rank it in one explicit live request.
- [`readable-workflows/support`](readable-workflows/support/README.md): one
  composed map followed by a confidence gate.
- [`readable-workflows/release`](readable-workflows/release/README.md): one
  release-risk map followed by an offline gate.
- [`readable-workflows/incident`](readable-workflows/incident/README.md):
  category map, `jq` catalog lookup into a native request field, second map,
  then gate.
- [`release-readiness`](release-readiness/review.sh): one bounded synthetic
  release-findings reduce and optional offline gate.
- [`code-smell-review`](code-smell-review/README.md): explicit bounded source
  files, one aggregate judgment, and an optional offline cohesion gate.
- [`todo-fulfillment`](todo-fulfillment/README.md): compare every prioritized
  todo item with the same complete code diff and preserve an explicit
  fulfilled/not-fulfilled/uncertain decision.
- [`unix-review-pipeline`](unix-review-pipeline/README.md): visible emitter,
  rank, map, jq, reduce, gate, and safe final projection pipeline.

The existing `support-routing`, `change-risk-gate`, and `issue-ranking`
directories remain low-level Bash references. They expose transport mechanics
for portability and are intentionally not workflow interpreters.
