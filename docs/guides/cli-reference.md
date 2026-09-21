# CLI reference

Run `jeq --help` or `jeq <command> --help` for the current flags. The command
surface is intentionally small:

| Command | Purpose | Network |
| --- | --- | --- |
| `ask` | Evaluate one request | Yes |
| `validate` | Validate request documents | No |
| `map` | Evaluate each input record | Yes |
| `rate` | Add a score judgment | Yes |
| `reduce` | Evaluate one aggregate judgment | Yes |
| `rank` | Order candidates by a choice judgment | Yes |
| `gate` | Apply pass/uncertain/reject thresholds | No |
| `models` | List available models | Yes |
| `examples` | Show workflow examples | No |
| `version` | Print build metadata | No |

JSON is the default document format. `map`, `rate`, `reduce`, `rank`, and `gate`
support NDJSON where their input is a stream; `ask` and `validate` use JSON
requests. `--verbose` writes structured trace events to stderr and never changes
stdout. For composed commands, precedence is `--model`, then `TYPESAFE_DEFAULT_MODEL`, then one config source: the file
named by non-empty `JEQ_CONFIG`, otherwise the default path
(`$XDG_CONFIG_HOME/jeq/config.json` or `$HOME/.config/jeq/config.json`). If
that optional default config is absent, use `jev-latest`. A native `ask` request's
embedded model is authoritative. `--trace-id` overrides `JEQ_TRACE_ID`; traces are stateless and
process-local unless the caller correlates them. Check exit codes in scripts:

| Code | Meaning |
| ---: | --- |
| 0 | Success |
| 1 | Authentication, API, network, timeout, or response failure |
| 2 | Invalid usage or local input (including unknown command/flag) |
| 10 | Gate reject |
| 11 | Gate uncertain decision |
| 130 | Interrupted |

Provider selection is explicit: `JEQ_PROVIDER=typesafe|vercel|custom`, or
`default_provider` in `JEQ_CONFIG`; there is no credential inference or fallback.
Named profiles use `base_url`, `default_model`, `auth`, and `api_key_env`.
Remote endpoints require HTTPS. Unauthenticated HTTP is loopback-only.
`JEQ_CONFIG` overrides the optional `$XDG_CONFIG_HOME/jeq/config.json`, then
`$HOME/.config/jeq/config.json`. Credentials and private state must remain
outside logs and arguments. The caller owns retries, persistence, redaction, and
actions; jeq never executes model output.
