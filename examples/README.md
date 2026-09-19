# Readable Unix pipelines

These examples use the composable primitives from ADR 0004. `gev map` performs
one named semantic judgment per record; `jq` projects, joins, and constructs
native requests; `gev gate` applies an explicit offline numeric policy.

The same JSON record remains the transport envelope. Before logging or sharing
it, project away sensitive state explicitly:

```sh
jq 'del(.customer_message, .details, ._gev.route)'
```

A live run needs `TYPESAFE_API_KEY` and spends account budget. Use a local fake
endpoint for development. Model evidence is data: examples validate it as a catalog key or a numeric value, and never execute it. `map` and `reduce` resolve models as `--model`, `TYPESAFE_DEFAULT_MODEL`, user config (`${XDG_CONFIG_HOME:-$HOME/.config}/gev/config.json`), then `jev-latest`; the config contains only `default_model`.


## Recommended pipelines

- [`readable-workflows/support`](readable-workflows/support/README.md): one
  composed map followed by a confidence gate.
- [`readable-workflows/release`](readable-workflows/release/README.md): one
  release-risk map followed by an offline gate.
- [`readable-workflows/incident`](readable-workflows/incident/README.md):
  category map, `jq` catalog lookup into a native request field, second map,
  then gate.
- [`code-smell-review`](code-smell-review/README.md): explicit bounded source
  files, one aggregate judgment, and an optional offline cohesion gate.

The existing `support-routing`, `change-risk-gate`, and `issue-ranking`
directories remain low-level Bash references. They expose transport mechanics
for portability and are intentionally not workflow interpreters.
