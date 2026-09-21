# Support routing: map then gate

The map asks one typed question. The gate is offline and inspects the response
confidence; it never calls the API.

From the repository root, run:

```sh
< examples/readable-workflows/support/fixtures/ticket.json \
jeq map --as route --questions examples/readable-workflows/support/questions.json --state-pointer /ticket |
  jeq gate --as route_policy \
    --value-pointer /_jeq/route/answers/route/confidence \
    --pass-min 0.70 --reject-max 0.40
```

Use `jq` before logs because map intentionally carries the complete original
record:

```sh
... | jq 'del(.ticket, ._jeq.route)'
```

The route answer is only evidence. A caller may map an allowlisted catalog key
to a fixed queue in `jq`; it must not turn model text into shell syntax.

Fixture: [`fixtures/ticket.json`](fixtures/ticket.json).
