# Architecture

## Purpose

`gev` is a command-line client for TypeSafe System One. Human interactions use
deterministic plain text through Cobra. Evaluation result streams use lossless
JSON/NDJSON so agents and Unix pipelines can compose them.

## Package arrows

Keep dependencies pointing inward. The domain does not know about Cobra, HTTP,
files, stdin, or renderers.

```text
cmd/gev (composition root)
  ├──> internal/cli       (commands and shell policy)
  ├──> internal/infra/render
  ├──> internal/infra/source
  └──> internal/infra/typesafeapi

internal/cli
  ├──> internal/domain/gev       (source rules and composition)
  ├──> internal/domain/contract  (request and response types)
  └──> ports supplied by cmd/gev

internal/infra/render
  ├──> internal/domain/contract
  └──> internal/domain/gev
internal/infra/source
  └──> internal/domain/gev
internal/infra/typesafeapi
  ├──> internal/domain/contract
  └──> internal/domain/gev
```

### Composition root

`cmd/gev/main.go` is the only production composition root. It wires:

- `internal/infra/source` to file and stdin readers;
- `internal/infra/typesafeapi` to an HTTP client;
- `internal/infra/render.JSON` to the CLI renderer port; and
- process environment, stdin, stdout, and stderr.

The CLI receives these dependencies through narrow ports. Unit tests can inject
bounded readers, fake clients, and renderers without touching the network or
filesystem.

### Domain

`internal/domain/contract` owns strict, lossless JSON decoding and deterministic
encoding. `internal/domain/gev` owns source conflicts, local validation, and
composed request construction. Stable error codes live in
`internal/domain/gev/codes`.

### Infrastructure

- `internal/infra/render` writes lossless JSON/NDJSON evaluation result streams.
  Human-facing commands and errors use deterministic plain text at the CLI boundary; TOON is not shipped.
- `internal/infra/source` reads only explicitly selected files or stdin and
  enforces bounded reads.
- `internal/infra/typesafeapi` is the only wire adapter. It owns bearer auth,
  bounded reads, documented retry statuses, timeout classification, and
  transport-error redaction.

## Command shape

| Command | Network | Output |
| --- | --- | --- |
| `gev` | No | Standard Cobra help |
| `gev examples` | No | Plain workflow discovery text |
| `gev version` | No | Plain build information |
| `gev models` | Yes | Plain model list |
| `gev validate` | No | Plain validation receipt |
| `gev ask` | Yes | JSON evaluation response |
| `gev map` / `gev reduce` / `gev gate` | Map/reduce: yes; gate: no | JSON/NDJSON result stream |
| `gev help` / `gev --help` | No | Cobra prose |
| `gev completion <shell>` | No | Shell script prose |

Exit classes are stable: `0` success, `1` auth/API/network/timeout failure,
`2` usage or local-input failure, `10` gate reject, `11` gate uncertain, and
`130` interruption. Cobra renders errors on stderr; failed runs do not emit an
error document on stdout. Retry progress also uses stderr.
