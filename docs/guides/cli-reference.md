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
| `version` | Print build metadata | No |

JSON is the default document format. NDJSON is the stream format. TOON is an
optional compact output format. `--verbose` writes structured trace events to
stderr and never changes stdout. Check exit codes in scripts; do not parse human
error wording. Credentials and private state must remain outside logs and
arguments.
