# Release risk: map then gate

Map asks for a numeric risk score. Gate applies explicit policy without another
network call. The command returns `0` for all pass records, `10` when any record
is rejected, and `11` when none are rejected but at least one is uncertain.

From the repository root in Bash, with `jeq` and `TYPESAFE_API_KEY` configured, run:

```bash
set -o pipefail
policy_status=0
if < examples/readable-workflows/release/fixtures/release.json \
  jeq map --as risk --questions examples/readable-workflows/release/questions.json --state-pointer /change |
  jeq gate --as release_policy \
    --value-pointer /_jeq/risk/answers/risk/score \
    --pass-min 0.80 --reject-max 0.30; then
  policy_status=0
else
  policy_status=$?
fi
case "$policy_status" in
  0) printf '%s\n' 'policy passed' ;;
  10) printf '%s\n' 'policy rejected' >&2 ;;
  11) printf '%s\n' 'policy is uncertain' >&2 ;;
  *) exit "$policy_status" ;;
esac
exit "$policy_status"
```

The original change is deliberately carried for the next operation. Use a
projection before logs or incident output:

```sh
... | jq 'del(.change, ._jeq.risk)'
```

For a bounded batch, `reduce` makes one request over the complete collection;
there is no iterative fold or hidden per-item request. The fixture is one NDJSON
record, so the framing is explicit:

```bash
< examples/readable-workflows/release/fixtures/release.json jeq reduce --as batch_risk \
  --questions examples/readable-workflows/release/questions.json --input ndjson --model jev-latest
```

The aggregate envelope retains `items` and the complete response under
`_jeq.batch_risk`. Project it before logs or sharing; map and reduce preserve
input state by design.

Fixture: [`fixtures/release.json`](fixtures/release.json).
