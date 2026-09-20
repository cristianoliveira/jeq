# Getting started

Create a request without a credential, then validate it locally:

```sh
cat > request.json <<'JSON'
{"state":{"text":"The payment is overdue"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}
JSON
jeq validate --request request.json
```

Obtain a credential through your normal secret manager. Export it for this
process without typing it into shell history, then evaluate:

In Bash, use a no-echo prompt (or your platform's secure secret manager):

```bash
read -rsp 'TypeSafe API key: ' TYPESAFE_API_KEY
printf '\n'
export TYPESAFE_API_KEY
jeq ask --request request.json
unset TYPESAFE_API_KEY
```

A successful response contains the resolved `model`, named `answers`, and `usage`,
for example:

```json
{"model":"jev-latest","answers":{"urgent":{"type":"noul","noul":0.9}},"usage":{"input_tokens":42,"output_tokens":8}}
```

`ask`, `map`, `rate`, `reduce`, and `rank` evaluate through TypeSafe and need
network access. Each primitive request costs usage. Review state and prompts,
keep them small and redacted, and never put credentials in requests, logs, or
shell history. Use `--verbose` for a trace on stderr; stdout remains pipeline
data. Check exit status in automation.
