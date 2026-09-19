# Purpose

`internal` contains JEQ's application implementation, separated into shell orchestration, domain capabilities, and infrastructure adapters.

# Boundaries

The [CLI](cli/AGENTS.md) is the shell ring. The [domain](domain/AGENTS.md) owns contracts, composition, and semantic enrichment. [Infrastructure](infra/AGENTS.md) owns files, output encoding, and HTTP. Test-only architecture, black-box, fixture, and support groupings remain under this guide rather than becoming runtime modules.

# Connections

- [CLI](cli/AGENTS.md): consumes domain capabilities and injected infrastructure ports.
- [Domain](domain/AGENTS.md): provides stable contracts and pure decision/enrichment operations.
- [Infrastructure](infra/AGENTS.md): provides implementations for external boundaries.

# Landmarks

- `internal/cli/run.go:RunWithDeps`: application entry boundary after process bootstrap.
- `internal/domain/contract/types.go:Request`: shared request value crossing shell and adapters.
- `internal/domain/pipeline/pipeline.go:Enrich`: semantic record-enrichment boundary.

# Boundary flows

- Information flow: `internal/cli/run.go:RunWithDeps` -> `internal/domain/jeq/compose.go:Compose` via `internal/cli/ask.go:NewAskCmd`; value: `jeq.ComposeInput`.
- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/infra/typesafeapi/client.go:Client.Evaluate` via `internal/cli/map.go:NewMapCmd`; value: `contract.Request`.
- Information flow: `internal/infra/typesafeapi/client.go:Client.Evaluate` -> `internal/cli/ask.go:NewAskCmd` via `internal/cli/ask.go:NewAskCmd`; value: `contract.Response`.

# Placement

Use the existing layer that owns a responsibility. Create a new internal module only for a cohesive capability with a distinct boundary, stable collaborators, and a dependency direction that does not make the domain depend on adapters.
