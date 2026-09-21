# Local map concurrency

Run a real `jeq map --input ndjson` workload against a loopback-only fake System One endpoint:

```sh
go install ./cmd/jeq
go run ./examples/map-concurrency --records 100
```

The example reports `records`, `requests`, safe peak in-flight requests, elapsed time,
order verification, and `PASS`. It requires `jeq` on `PATH`, makes no live calls,
and treats elapsed time as informational. A successful run proves peak concurrency is
between 2 and 4 and that every output record remains in input order.
