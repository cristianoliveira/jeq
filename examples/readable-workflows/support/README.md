# Support routing: map then gate

The map asks one typed question. The gate is offline and inspects the response
confidence; it never calls the API.

```sh
GEV_BIN=${GEV_BIN:-gev}
"$GEV_BIN" map --as route --questions questions.json --state-pointer /ticket |
  "$GEV_BIN" gate --as route_policy \
    --value-pointer /_gev/route/answers/route/confidence \
    --pass-min 0.70 --reject-max 0.40
```

Use `jq` before logs because map intentionally carries the complete original
record:

```sh
... | jq 'del(.ticket, ._gev.route)'
```

The route answer is only evidence. A caller may map an allowlisted catalog key
to a fixed queue in `jq`; it must not turn model text into shell syntax.

Fixture: [`fixtures/ticket.json`](fixtures/ticket.json).
