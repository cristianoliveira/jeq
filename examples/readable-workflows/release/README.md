# Release risk: map then gate

Map asks for a numeric risk score. Gate applies explicit policy without another
network call. The command returns `0` for all pass records, `10` when any record
is rejected, and `11` when none are rejected but at least one is uncertain.

From the repository root, run:

```sh
< examples/readable-workflows/release/fixtures/release.json \
jeq map --as risk --questions examples/readable-workflows/release/questions.json --state-pointer /change |
  jeq gate --as release_policy \
    --value-pointer /_jeq/risk/answers/risk/score \
    --pass-min 0.80 --reject-max 0.30
```

The original change is deliberately carried for the next operation. Use a
projection before logs or incident output:

```sh
... | jq 'del(.change, ._jeq.risk)'
```

For a bounded batch, `reduce` makes one request over the complete collection;
there is no iterative fold or hidden per-item request:

```sh
< examples/readable-workflows/release/fixtures/release.json | jeq reduce --as batch_risk \
  --questions examples/readable-workflows/release/questions.json --input ndjson --model jev-latest
```

The aggregate envelope retains `items` and the complete response under
`_jeq.batch_risk`. Project it before logs or sharing; map and reduce preserve
input state by design.

Fixture: [`fixtures/release.json`](fixtures/release.json).
