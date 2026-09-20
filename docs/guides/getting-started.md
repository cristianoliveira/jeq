# Getting started

Export the credential only in your current environment. Validate the request
locally before paying for an evaluation:

```sh
export TYPESAFE_API_KEY='replace-me'
jeq validate --request request.json
```

Ask one typed question about explicit state:

```sh
cat > request.json <<'JSON'
{"state":{"text":"The payment is overdue"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}
JSON
jeq ask --request request.json
```

`ask`, `map`, `rate`, `reduce`, and `rank` evaluate through TypeSafe and need
network access plus `TYPESAFE_API_KEY`. `validate` checks request shape locally.
The result contains the model, named answers, usage, and command evidence. Each
primitive request costs usage; batching changes request shape and cost, not the
need to review state and prompts. Use `--verbose` to send a trace to stderr;
stdout remains pipeline data. Never put credentials or private state in requests,
logs, or shell history.

Start with small, redacted state. Each request costs usage. Set explicit timeouts
and retry limits for automation. Check the exit status: ordinary input and
transport failures are non-zero, and policy `gate` reports pass, ambiguous, or
reject separately.
