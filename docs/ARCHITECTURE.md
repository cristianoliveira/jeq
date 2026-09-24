# Architecture

## Purpose

`jeq` is a command-line client for TypeSafe System One. Human interactions use
deterministic plain text through Cobra. Evaluation result streams use lossless
JSON/NDJSON so agents and Unix pipelines can compose them.

## Package arrows

Keep dependencies pointing inward. The domain does not know about Cobra, HTTP,
files, stdin, or renderers.

```text
cmd/jeq (composition root)
  ├──> internal/cli       (commands and shell policy)
  ├──> internal/infra/render
  ├──> internal/infra/source
  └──> internal/infra/typesafeapi

internal/cli
  ├──> internal/domain/jeq       (source rules and composition)
  ├──> internal/domain/contract  (request and response types)
  └──> ports supplied by cmd/jeq

internal/infra/render
  ├──> internal/domain/contract
  └──> internal/domain/jeq
internal/infra/source
  └──> internal/domain/jeq
internal/infra/typesafeapi
  ├──> internal/domain/contract
  └──> internal/domain/jeq
```

### Composition root

`cmd/jeq/main.go` is the only production composition root. It wires:

- `internal/infra/source` to file and stdin readers;
- `internal/infra/typesafeapi` to an HTTP client;
- `internal/infra/render.JSON` to the CLI renderer port; and
- process environment, stdin, stdout, and stderr.

The CLI receives these dependencies through narrow ports. Unit tests can inject
bounded readers, fake clients, and renderers without touching the network or
filesystem.

### Statelessness invariant

jeq is stateless between invocations. It does not create, discover, read, or
update memory, session, history, run-state, or trace files. A later invocation
can observe earlier evidence only when the caller explicitly supplies that
evidence through stdin or a selected input file. User configuration is
read-only configuration, not operational memory.

Evaluation evidence belongs on stdout. Diagnostics and opt-in debugging logs
belong on stderr. jeq may emit logs for the lifetime of the current process, but
it never persists them itself. A caller may deliberately redirect stderr to a
file; ownership, retention, and reuse of that file remain outside jeq.

New features must not add implicit local persistence or automatic discovery of
previous runs. Any proposal to persist state requires a new explicit product
decision and architecture review; it must not enter as an implementation detail
of observability, retries, composition, or convenience.

### Domain

`internal/domain/contract` owns strict, lossless JSON decoding and deterministic
encoding. `internal/domain/jeq` owns source conflicts, local validation, and
composed request construction. Stable error codes live in
`internal/domain/jeq/codes`.

### Infrastructure

- `internal/infra/render` writes lossless JSON/NDJSON evaluation result streams.
  Human-facing commands and errors use deterministic plain text at the CLI boundary; TOON is not shipped.
- `internal/infra/source` reads only explicitly selected files or stdin and
  enforces bounded reads.
- `internal/infra/typesafeapi` is the only wire adapter. It owns bearer auth,
  bounded reads, documented retry statuses, timeout classification, and
  transport-error redaction.

## Ephemeral execution tracing

`--verbose` is a global long-only opt-in. It writes versioned `jeq.trace.v1` lifecycle metadata as single-line JSON to stderr; stdout remains the sole semantic evidence stream. `--trace-id` takes precedence over `JEQ_TRACE_ID` and accepts only bounded safe identifiers for caller-owned cross-process correlation. Events never persist, discover, or update files and never contain prompts, state, credentials, headers, URLs with user data, or provider bodies. The trace describes execution metadata, not model reasoning. Without an explicit trace ID, events remain process-local.

## Environment-only infrastructure resolution

Infrastructure configuration is process-wide and comes from the environment or the strict JSON config; commands do not expose infrastructure flags. Select `JEQ_PROVIDER=typesafe|vercel|custom`, or `default_provider` in `JEQ_CONFIG`. Vercel uses `https://ai-gateway.vercel.sh/typesafe`, `AI_GATEWAY_API_KEY` (then `VERCEL_OIDC_TOKEN`), and `typesafe-ai/jev`. Custom profiles name `base_url`, `default_model`, `auth`, and `api_key_env`; credentials are never stored in config. Remote endpoints require HTTPS; unauthenticated HTTP is loopback-only. `JEQ_CONFIG` overrides optional `$XDG_CONFIG_HOME/jeq/config.json` and `$HOME/.config/jeq/config.json`; there is no provider inference or fallback.

For composed commands, one shared resolver applies `--model`, `JEQ_DEFAULT_MODEL`, the selected profile `default_model`, top-level config `default_model`, legacy `TYPESAFE_DEFAULT_MODEL`, then the `jev-latest` fallback. Native requests keep their embedded model authoritative; `ask --request` and `validate --request` reject an explicit `--model`. All network commands use the selected profile for `/v1/systemone` and `/v1/models`; the existing adapter owns bounded retries and redirect refusal.

## Command shape

| Command | Network | Output |
| --- | --- | --- |
| `jeq` | No | Standard Cobra help |
| `jeq examples` | No | Plain workflow discovery text |
| `jeq version` | No | Plain build information |
| `jeq models` | Yes | Plain model list |
| `jeq validate` | No | Plain validation receipt |
| `jeq ask` | Yes | JSON evaluation response |
| `jeq map` / `jeq reduce` / `jeq gate` / `jeq rank` / `jeq rate` | Map/reduce/rank/rate: yes; gate: no | JSON/NDJSON result stream |
| `jeq help` / `jeq --help` | No | Cobra prose |
| `jeq completion <shell>` | No | Shell script prose |

Exit classes are stable: `0` success, `1` auth/API/network/timeout failure,
`2` usage or local-input failure, `10` gate reject, `11` gate uncertain, and
`130` interruption. Cobra renders errors on stderr; failed runs do not emit an
error document on stdout. Retry progress also uses stderr.
