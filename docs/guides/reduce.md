# Reduce a bounded collection

Use `reduce` when one judgment must see a complete, bounded collection. A
release review is a useful example: send all synthetic findings once and ask a
positive question, `release_ready`, so a high Noul value means ready to ship.
Do not use `reduce` for unbounded streams, per-record decisions, or relative
ordering.

```sh
./examples/release-readiness/review.sh
```

`reduce` makes one TypeSafe request for the complete NDJSON collection. Keep the
collection small enough to review, control cost, and avoid sending private data.
JSON and NDJSON inputs are supported according to the selected command input
mode. The aggregate evidence is appended under:

```text
/_jeq/release_ready/answers/release_ready/noul
```

The example's output can be sent to an offline policy gate. These thresholds are
illustrative, not calibrated defaults:

```sh
./examples/release-readiness/review.sh \
  | jeq gate --as release_policy \
    --value-pointer /_jeq/release_ready/answers/release_ready/noul \
    --pass-min 0.80 --reject-max 0.40
```

Values at or above `0.80` pass, values at or below `0.40` reject, and the middle
is uncertain. `gate` makes no network request.

Choose the primitive that matches the question:

| Primitive | Evidence shape | Request behavior |
| --- | --- | --- |
| `map` | One judgment per record | One request per record |
| `rate` | One score per record | One request per record |
| `rank` | Relative candidate order | One bounded request |
| `reduce` | One judgment over the collection | One bounded request |
| `gate` | Offline threshold decision | No request |

The script is intentionally singular: it reads synthetic findings, runs exactly
one reduce, and performs no deployment or release action. It preserves jeq's
stdout, stderr, and exit status. Set `jeq` to a fake executable for offline
checks or to a reviewed jeq binary for a real request.
