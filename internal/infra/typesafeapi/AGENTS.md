# Purpose

The TypeSafe API adapter owns HTTP communication with System One and models endpoints, including authentication, bounded response handling, retry behavior, and transport error classification.

# Boundaries

It implements the evaluator and model-fetching behavior expected by the CLI and pipeline. It does not compose requests, decide shell source precedence, or render command output.

# Connections

- [Domain](../../domain/AGENTS.md): supplies contract values and stable jeq errors.
- [Pipeline](../../domain/pipeline/AGENTS.md): consumes this package as its `pipeline.Evaluator` implementation.
- [CLI](../../cli/AGENTS.md): consumes the adapter through injected API ports.
- [Infrastructure](../AGENTS.md): owns this adapter's external-boundary placement.

# Landmarks

- `internal/infra/typesafeapi/client.go:New`: constructs a configured API client.
- `internal/infra/typesafeapi/client.go:Client.Evaluate`: posts one System One request and decodes its response.
- `internal/infra/typesafeapi/client.go:Client.Models`: fetches the account's available models.

# Boundary flows

- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/infra/typesafeapi/client.go:Client.Evaluate` via `internal/cli/map.go:NewMapCmd`; value: `contract.Request`.
- Information flow: `internal/infra/typesafeapi/client.go:Client.Evaluate` -> `internal/cli/ask.go:NewAskCmd` via `internal/cli/ask.go:NewAskCmd`; value: `contract.Response`.

# Placement

Keep TypeSafe-specific HTTP policy here. Add another adapter module only for a distinct external protocol; keep shared document semantics in domain contract.
