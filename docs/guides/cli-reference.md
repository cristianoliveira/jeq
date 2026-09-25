# CLI reference

Provider setup and endpoint constraints are covered in the [provider guide](providers.md).

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
requests.

## Input flags

Input flags keep one meaning across commands:

| Flag | Meaning |
| --- | --- |
| `--state TEXT` | Literal text state |
| `--state-file PATH` | Text file, or `-` for stdin |
| `--state-json JSON` | Inline JSON state value |
| `--state-json-file PATH` | JSON file, or `-` for stdin |
| `--questions PATH` | Questions document file, or `-` for stdin |
| `--questions-json JSON` | Inline questions document for `ask`, `validate`, `map`, and `reduce` |
| `--request PATH` | Complete native request file, or `-` for stdin |

`rate` and `rank` build their own question shapes and do not accept question-document flags. Each command reads stdin at most once; use a file for the other sources. Inline values can appear in shell history, so use file flags or stdin when that exposure matters. `--state-json` now means inline JSON. Migrate previous `--state-json FILE` invocations to `--state-json-file FILE`; jeq does not guess whether an argument is a path. `--verbose` writes structured trace events to stderr and never changes stdout.
Composed model precedence is `--model`, `JEQ_DEFAULT_MODEL`, the selected
provider profile's `default_model`, top-level config `default_model`, legacy
`TYPESAFE_DEFAULT_MODEL`, then the `jev-latest` fallback. `JEQ_CONFIG` names an
explicit config file; otherwise jeq reads the optional
`$XDG_CONFIG_HOME/jeq/config.json` or `$HOME/.config/jeq/config.json`. If that
optional file is absent, resolution continues to the legacy environment value
and fallback. Native `ask --request` and `validate --request` documents own
their model; an explicit `--model` is rejected, so edit the request document to
change it. `--trace-id` overrides `JEQ_TRACE_ID`; traces are stateless and
process-local unless the caller correlates them. Check exit codes in scripts:

| Code | Meaning |
| ---: | --- |
| 0 | Success |
| 1 | Authentication, API, network, timeout, or response failure |
| 2 | Invalid usage or local input (including unknown command/flag) |
| 10 | Gate reject |
| 11 | Gate uncertain decision |
| 130 | Interrupted |

`gate` requires both thresholds and enforces `0 <= reject-max < pass-min <= 1`.
Comparisons are inclusive: `value >= pass-min` passes and `value <= reject-max`
rejects. It emits every processed record, including rejects and uncertain
records. Its aggregate status is 10 if any record rejects; otherwise it is 11 if
any record is uncertain; reject takes precedence. Filtering and actions remain
the caller's job. In Bash pipelines, use `set -o pipefail`; it returns the
rightmost failing stage's status, not every stage's exact error. Read
`PIPESTATUS` immediately only when a workflow needs each stage's status.

### Usage summary

Pass `--usage-summary` to `ask`, `map`, `rate`, `rank`, or `reduce` to append one
JSON line to stderr. The flag is opt-in. stdout and the command's exit code keep
their normal meanings, so JSON and NDJSON pipelines remain composable. The
summary reports `attempted_requests`, `successful_requests`, `processed_records`,
`decoded_answers`, provider-reported `input_tokens` and `output_tokens`,
`answers_per_1000_input_tokens`, `elapsed_ms`, and distinct
`resolved_model_versions` returned by the provider.

The ratio is decoded answers divided by input tokens, multiplied by 1,000. It is
`null` when the provider reports zero or fewer input tokens; raw counts remain
available. Attempts count actual HTTP exchanges, including retries. Successful
requests count responses that jeq decoded; decoded answers count only judgments
attached to their requested state or record. For streams, processed records
count records the command started processing, not output lines. Usage totals
include only valid responses observed by jeq. A provider may bill a failed or
cancelled request even when it returns no usage, so the summary cannot account
for such charges. jeq makes no extra calls, estimates no prices, and excludes
prompts, state, and credentials.

Provider selection is explicit: `JEQ_PROVIDER=typesafe|vercel|custom`, or
`default_provider` in `JEQ_CONFIG`; there is no credential inference or fallback.
Named profiles use `base_url`, `default_model`, `auth`, and `api_key_env`.
Remote endpoints require HTTPS. Unauthenticated HTTP is loopback-only.
`JEQ_CONFIG` overrides the optional `$XDG_CONFIG_HOME/jeq/config.json`, then
`$HOME/.config/jeq/config.json`. Credentials and private state must remain
outside logs and arguments. The caller owns retries, persistence, redaction, and
actions; jeq never executes model output.
