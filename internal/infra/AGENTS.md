# Purpose

Infrastructure owns JEQ's external adapters: explicit source reading, machine-output rendering, and the TypeSafe HTTP client.

# Boundaries

Source and render are implementation groupings covered here: source touches files and stdin, while render writes output. [TypeSafe API](typesafeapi/AGENTS.md) is the only wire adapter. The domain remains independent and the [CLI](../cli/AGENTS.md) receives adapters through composition wiring.

# Connections

- [CLI](../cli/AGENTS.md): consumes adapter behavior through injected ports.
- [Domain](../domain/AGENTS.md): supplies contracts and stable errors consumed by adapters.
- [TypeSafe API](typesafeapi/AGENTS.md): provides the evaluator and models client.

# Landmarks

- `internal/infra/source/source.go:ReadFile`: reads bounded explicit file input.
- `internal/infra/render/json.go:JSON`: writes deterministic machine output.
- `internal/infra/typesafeapi/client.go:Client.Evaluate`: evaluates a request through TypeSafe.
- `internal/infra/typesafeapi/client.go:Client.Models`: retrieves available models.

# Boundary flows

- Information flow: `internal/cli/ask.go:NewAskCmd` -> `internal/infra/source/source.go:ReadFile` via `internal/cli/ask.go:NewAskCmd`; value: `[]byte`.
- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/infra/typesafeapi/client.go:Client.Evaluate` via `internal/cli/map.go:NewMapCmd`; value: `contract.Request`.
- Information flow: `internal/infra/typesafeapi/client.go:Client.Evaluate` -> `internal/infra/render/json.go:JSON` via `internal/cli/ask.go:NewAskCmd`; value: `contract.Response`.

# Placement

Put operating-system, serialization-output, and network concerns here. Keep each adapter behind the narrow port owned by its consumer; do not move composition wiring out of the command package.
