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

JEQ is stateless between invocations. It does not create, discover, read, or
update memory, session, history, run-state, or trace files. A later invocation
can observe earlier evidence only when the caller explicitly supplies that
evidence through stdin or a selected input file. User configuration is
read-only configuration, not operational memory.

Evaluation evidence belongs on stdout. Diagnostics and opt-in debugging logs
belong on stderr. JEQ may emit logs for the lifetime of the current process, but
it never persists them itself. A caller may deliberately redirect stderr to a
file; ownership, retention, and reuse of that file remain outside JEQ.

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

## Environment-only infrastructure resolution

Infrastructure configuration is process-wide and comes from the environment; the affected commands do not expose infrastructure flags. `TYPESAFE_BASE_URL` selects the API root and must be an absolute `http` or `https` URL. It is validated before API-key lookup or HTTP-client construction. `TYPESAFE_API_KEY` supplies credentials. `JEQ_CONFIG`, when explicitly set and nonblank, is always read and strictly validated (including size, syntax, duplicate keys, and unsupported fields) before model selection. Without an explicit config path, optional discovery checks `XDG_CONFIG_HOME` and then `$HOME/.config/jeq/config.json`, but that optional file is not read when `--model` or `TYPESAFE_DEFAULT_MODEL` already supplies a higher-precedence model.

Model precedence is `--model`, then `TYPESAFE_DEFAULT_MODEL`, then the validated config model, then the built-in default. Native `ask` requests keep their embedded request model; composed `ask`, `map`, `reduce`, `rank`, and `rate` resolve the model through this precedence chain. Thus native request data is authoritative, while composed commands use CLI/environment/config model selection.

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
