# Support routing: map then gate

The map asks one typed question. The gate is offline and inspects the response
confidence; it never calls the API.

From the repository root in Bash, with `jeq` and `TYPESAFE_API_KEY` configured, run:

```bash
set -o pipefail
policy_status=0
if < examples/readable-workflows/support/fixtures/ticket.json \
  jeq map --as route --questions examples/readable-workflows/support/questions.json --state-pointer /ticket |
  jeq gate --as route_policy \
    --value-pointer /_jeq/route/answers/route/confidence \
    --pass-min 0.70 --reject-max 0.40; then
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

Use `jq` before logs because map intentionally carries the complete original
record:

```sh
... | jq 'del(.ticket, ._jeq.route)'
```

The route answer is only evidence. A caller may map an allowlisted catalog key
to a fixed queue in `jq`; it must not turn model text into shell syntax.

Fixture: [`fixtures/ticket.json`](fixtures/ticket.json).
