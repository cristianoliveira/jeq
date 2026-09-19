# Purpose

Pipeline owns append-only semantic enrichment of JSON records: per-record map, collection reduce, JSON Pointer selection, and offline gate decisions.

# Boundaries

It accepts validated domain values and an evaluator port. It does not read streams, render output, perform HTTP, or parse command flags.

# Connections

- [Contract](../contract/AGENTS.md): supplies request and response types and validation.
- [JEQ rules](../jeq/AGENTS.md): supplies stable errors used by pipeline failures.
- [CLI](../../cli/AGENTS.md): orchestrates framing, limits, evaluation, and output around pipeline calls.
- [TypeSafe adapter](../../infra/typesafeapi/AGENTS.md): implements the evaluator port consumed by map and reduce.

# Landmarks

- `internal/domain/pipeline/pipeline.go:Enrich`: appends one evaluation response under `_jeq`.
- `internal/domain/pipeline/reduce.go:Reduce`: evaluates one bounded collection.
- `internal/domain/pipeline/gate.go:Gate`: appends a deterministic pass, reject, or uncertain receipt.
- `internal/domain/pipeline/pipeline.go:Select`: selects a value by JSON Pointer.

# Boundary flows

- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/infra/typesafeapi/client.go:Client.Evaluate` via `internal/cli/map.go:NewMapCmd`; value: `contract.Request`.
- Information flow: `internal/domain/pipeline/gate.go:Gate` -> `internal/cli/gate.go:NewGateCmd` via `internal/cli/gate.go:NewGateCmd`; value: `GateDecision`.

# Placement

Place reusable record transformations and offline policy semantics here. Keep command framing in CLI and evaluator implementations in infrastructure.
