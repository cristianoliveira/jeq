# Purpose

JEQ rules own source-mode validation, state composition, and stable error-code presentation for the application.

# Boundaries

This package is pure Go. It receives resolved bytes and values, never reads files or environment, and never knows Cobra or HTTP. The small `codes` registry is an implementation leaf of this capability.

# Connections

- [Contract](../contract/AGENTS.md): supplies document decoding and validation used while composing requests.
- [Pipeline](../pipeline/AGENTS.md): consumes stable JEQ errors for semantic operations.
- [CLI](../../cli/AGENTS.md): invokes source checks and composition after shell inputs are resolved.
- [Infrastructure](../../infra/AGENTS.md): uses stable errors when adapters classify external failures.

# Landmarks

- `internal/domain/jeq/sources.go:CheckSources`: validates native versus composed source modes.
- `internal/domain/jeq/compose.go:Compose`: builds a validated request from resolved inputs.
- `internal/domain/jeq/jeq.go:NewError`: creates a stable coded error.

# Boundary flows

- Information flow: `internal/cli/ask.go:NewAskCmd` -> `internal/domain/contract/types.go:Request` via `internal/domain/jeq/compose.go:Compose`; value: `contract.Request`.

# Placement

Place cross-command source and composition rules here. Keep document schema in contract and record enrichment or numeric policy in pipeline.
