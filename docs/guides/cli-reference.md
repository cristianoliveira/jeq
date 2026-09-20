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
| `gate` | Apply pass/ambiguous/reject thresholds | No |
| `models` | List available models | Yes |
| `examples` | Show workflow examples | No |
| `version` | Print build metadata | No |

JSON is the default document format. `map`, `rate`, `reduce`, `rank`, and `gate`
support NDJSON where their input is a stream; `ask` and `validate` use JSON
requests. TOON is an optional compact output format on commands that expose
`--output`. `--verbose` writes structured trace events to stderr and never
changes stdout. `--model` wins over `TYPESAFE_DEFAULT_MODEL`, then validated
`JEQ_CONFIG`, then the built-in default; native request models remain
authoritative. `--trace-id` overrides `JEQ_TRACE_ID`; traces are stateless and
process-local unless the caller correlates them. Check exit codes in scripts:

| Code | Meaning |
| ---: | --- |
| 0 | Success |
| 1 | Evaluation, input, transport, or output failure |
| 2 | CLI usage error |
| 10 | Gate reject |
| 11 | Gate ambiguous |
| 130 | Interrupted |

Credentials and private state must remain outside logs and arguments. The caller
owns retries, persistence, redaction, and actions; JEQ never executes model
output.
