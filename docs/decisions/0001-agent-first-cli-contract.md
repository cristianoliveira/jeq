# 0001. Agent-first CLI contract

- Status: Accepted
- Date: 2026-09-19

## Context

TypeSafe System One callers currently need to integrate the HTTP API or an SDK in each project. We want one shell interface that LLM agents can invoke without adding application code.

The CLI must preserve TypeSafe's native request capabilities while remaining deterministic, compact, discoverable, and safe for non-interactive automation.

## Decision

### Product direction

- Name the executable `gev`.
- Build it in Go with Cobra.
- Serve both humans and agents: concise human discovery/diagnostics, lossless JSON for composable result streams.
- Accept JSON documents only in version 1.
- Never prompt or read stdin implicitly.

### Commands

```text
gev ask        Send one System One request
gev map        Enrich JSON/NDJSON records with one named judgment
gev reduce     Aggregate one bounded JSON/NDJSON collection with one judgment
gev gate       Apply an offline numeric policy to JSON/NDJSON records
gev validate   Validate request input without credentials or network access
gev models     List models available to the account
gev version    Return version information
gev completion Generate Cobra shell completions
```

Noul, Choice, and Score remain question types in request data. They are not separate version 1 commands. `gev ask --request` is the native passthrough, so a separate API command is unnecessary.

With no arguments, `gev` returns a compact, deterministic offline plain-text home view containing its identity, purpose, credential readiness, default model, available commands, and one relevant next step. `gev examples [id]` is the primary self-contained workflow discovery path.

### Request modes

A caller provides either a complete native request:

```sh
gev ask --request request.json
gev ask --request - < request.json
```

or reusable questions plus exactly one state source:

```sh
gev ask --questions questions.json --state "Customer message"
gev ask --questions questions.json --state-file message.txt
gev ask --questions questions.json --state-json state.json
gev ask --questions questions.json --state-json - < state.json
```

The state flags mean:

- `--state`: literal text;
- `--state-file`: file or explicit stdin passed as text;
- `--state-json`: file or explicit stdin parsed as structured JSON.

Request modes and state sources never merge implicitly. Conflicts fail before credential lookup, filesystem side effects, or network access.

### Configuration

- Read authentication only from `TYPESAFE_API_KEY`; never accept an API key flag.
- Resolve `map`/`reduce` models from `--model`, then `TYPESAFE_DEFAULT_MODEL`, then strict user config `${XDG_CONFIG_HOME:-$HOME/.config}/gev/config.json`, then `jev-latest`; config contains only `default_model` and `--config` overrides its path.
- Preserve the existing `ask`/`validate` model-resolution behavior.
- Allow `TYPESAFE_BASE_URL` or `--base-url` for the API root.
- Use a 10-second default timeout.
- Retry only documented `429` and `529` responses with bounded backoff and `Retry-After` support.

### Output

- `ask`, `map`, `reduce`, and `gate` success output is lossless JSON/NDJSON with one trailing newline per document/record.
- Home, `examples`, `version`, `models`, and successful `validate` output is concise deterministic plain text on stdout.
- Help is standard Cobra plain text on stdout. There is no global output-format flag.
- Errors and diagnostics use Cobra's standard concise plain text on stderr; stdout remains empty on failure. Domain failures preserve symbolic codes and exit classification without exposing credentials, stack traces, or raw dependency failures.
- Keep output deterministic: no TTY-dependent layout, timestamps, locale-dependent values, or color.
- Preserve the resolved model, every answer, probabilities, confidence where available, score legends, and token usage in result streams.

### Errors

Use this stable exit classification:

```text
0   Success, regardless of confidence or probability (and all gate records pass)
1   Authentication, API, network, timeout, or response failure
2   Usage or locally invalid input
10  Gate policy: at least one record is rejected
11  Gate policy: no rejects, but at least one record is uncertain
130 Interrupted
```

Errors include a stable symbolic code and, when possible, one actionable recovery instruction. Unknown commands and flags identify valid alternatives.

Low confidence is not an error for `ask` or `map`. `gate` is the explicit offline
policy primitive: it owns only numeric pass/reject/uncertain statuses (10/11),
never calls the API, and does not reinterpret model text as an action.

## Consequences

- Agents can use the complete TypeSafe request contract without an SDK integration.
- Reusable question files separate stable judgments from changing application state.
- Explicit input sources prevent blocking and ambiguous precedence.
- The CLI must maintain strict result-stream JSON tests and stable human-output/error tests.
- YAML, interactive credential storage, and a TUI remain outside version 1; `map`, one-call `reduce`, and offline numeric `gate` are the supported primitives.
