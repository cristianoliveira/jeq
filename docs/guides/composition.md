# Composition

JEQ commands compose through JSON or NDJSON. Keep one record per line for streams.
Use `set -o pipefail` so a failed producer or policy cannot be hidden by a later
successful command. `jq` owns deterministic selection and reshaping; JEQ owns
judgment and typed evidence; the shell owns sequencing and final actions.

Native `ask` sends one complete request. Composed `map`, `rate`, `reduce`, and
`rank` construct a request from flags and input records; each primitive has a
clear request and usage cost. Make stdin explicit with `--input` and redirect
or pipe deliberately. See the [worked examples](../../examples/README.md).


```sh
cat records.ndjson \
  | jeq map --as urgency --input ndjson --questions-json "$QUESTIONS" \
  | jeq gate --as policy --input ndjson --value-pointer /urgency/score \
    --pass-min 0.8 --reject-max 0.2
```

Use `map` for independent records, `rate` for a score, `reduce` for one
aggregate judgment, and `rank` to order candidates. Use `gate` for a local
policy decision after evidence exists. Pointers select fields; inspect the
shape before choosing them.

Use `--output json`, `--output ndjson`, or `--output toon` where supported. Keep
stdout for data and stderr for diagnostics/traces. A model choice is evidence,
not permission to execute a command. The caller owns deduplication, persistence,
redaction, retries, and final action.
