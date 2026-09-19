# Architecture

## Purpose

`gev` is an agent-first command-line client for TypeSafe System One. The
binary emits deterministic JSON documents on stdout. Help and shell completion
are the only prose output exceptions.

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

- `internal/infra/render` writes one JSON document and one trailing newline.
  See ADR 0003: version 1 is JSON-only; TOON is not shipped.
- `internal/infra/source` reads only explicitly selected files or stdin and
  enforces bounded reads.
- `internal/infra/typesafeapi` is the only wire adapter. It owns bearer auth,
  bounded reads, documented retry statuses, timeout classification, and
  transport-error redaction.

## Command shape

| Command | Network | Output |
| --- | --- | --- |
| `gev` | No | JSON home document |
| `gev version` | No | JSON build information |
| `gev models` | Yes | JSON model list |
| `gev validate` | No | JSON validation receipt |
| `gev ask` | Yes | JSON evaluation response |
| `gev help` / `gev --help` | No | Cobra prose |
| `gev completion <shell>` | No | Shell script prose |

Exit classes are stable: `0` success, `1` auth/API/network/timeout failure,
`2` usage or local-input failure, and `130` interruption. Non-zero operational
runs emit one structured error document on stdout. Diagnostics and retry
progress use stderr only.
