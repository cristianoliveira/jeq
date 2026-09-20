# Issue ranking

This workflow reads one issue object per NDJSON line and makes exactly one jeq
request per valid line. It emits one correlated NDJSON line in input order:
`id`, `order`, the two score values, both score confidences, usage, and a
deterministic `rank_score`. Keeping this judgment evidence makes cost and
confidence auditable without copying raw issue state. Downstream tools can sort
explicitly, for example:

```sh
./rank.sh < fixtures/issues.ndjson | jq -s 'sort_by(-.rank_score)'
```

The example chooses **fail-fast** semantics. The first malformed input, missing
ID, operational jeq error, or unusable score stops the stream. Prior output is
kept, the error document or receipt is emitted, and no later records are sent.
This makes request count and spend predictable. A valid input with `n` lines
makes exactly `n` paid evaluation requests.

```sh
JEQ_BIN=jeq JEQ_MODEL=jev-latest \
  ./rank.sh < fixtures/issues.ndjson
```

Set `TYPESAFE_BASE_URL` for a local fake server or an explicitly approved live
endpoint. Each line is a separate paid evaluation when pointed at production;
use small synthetic fixtures and a pinned model. The script never evaluates or
executes any model-produced value.
