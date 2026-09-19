# Release risk: map then gate

Map asks for a numeric risk score. Gate applies explicit policy without another
network call. The command returns `0` for all pass records, `10` when any record
is rejected, and `11` when none are rejected but at least one is uncertain.

```sh
GEV_BIN=${GEV_BIN:-gev}
"$GEV_BIN" map --as risk --questions questions.json --state-pointer /change |
  "$GEV_BIN" gate --as release_policy \
    --value-pointer /_gev/risk/answers/risk/score \
    --pass-min 0.80 --reject-max 0.30
```

The original change is deliberately carried for the next operation. Use a
projection before logs or incident output:

```sh
... | jq 'del(.change, ._gev.risk)'
```

Fixture: [`fixtures/release.json`](fixtures/release.json).
