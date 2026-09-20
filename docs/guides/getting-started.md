# Getting started

Ask one typed question about explicit state:

```sh
cat > request.json <<'JSON'
{"state":{"text":"The payment is overdue"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}
JSON
jeq ask --request request.json
```

`ask`, `map`, `rate`, `reduce`, and `rank` evaluate through TypeSafe and need
network access plus `TYPESAFE_API_KEY`. `validate` checks request shape locally.
Use `--verbose` to send a trace to stderr; stdout remains pipeline data.

Start with small, redacted state. Each request costs usage. Set explicit timeouts
and retry limits for automation. Check the exit status: ordinary input and
transport failures are non-zero, and policy `gate` reports pass, ambiguous, or
reject separately.
