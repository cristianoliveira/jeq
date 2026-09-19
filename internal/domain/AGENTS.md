# Purpose

The domain owns the language of JEQ requests and responses, local validation and composition, and append-only judgment pipelines. It remains independent of shell, network, filesystem, and output adapters.

# Boundaries

[Contract](contract/AGENTS.md) defines lossless TypeSafe documents. [JEQ rules](jeq/AGENTS.md) define source selection, composition, and stable errors. [Pipeline](pipeline/AGENTS.md) defines map, reduce, and gate enrichment. The small `codes`, `arch`, `blackbox`, and fixture support groupings stay documented by their owning guides.

# Connections

- [CLI](../cli/AGENTS.md): consumes domain operations and supplies orchestration and ports.
- [Contract](contract/AGENTS.md): provides typed request, response, question, and model values.
- [JEQ rules](jeq/AGENTS.md): provides source and composition decisions and stable errors.
- [Pipeline](pipeline/AGENTS.md): consumes contracts and JEQ errors to enrich records.
- [Infrastructure](../infra/AGENTS.md): consumes contracts and domain errors at external boundaries.

# Landmarks

- `internal/domain/contract/types.go:Request`: native evaluation request value.
- `internal/domain/jeq/compose.go:Compose`: pure request composition entry point.
- `internal/domain/pipeline/pipeline.go:Enrich`: one-record map operation.
- `internal/domain/pipeline/reduce.go:Reduce`: one-call collection operation.
- `internal/domain/pipeline/gate.go:Gate`: offline evidence policy operation.

# Boundary flows

- Information flow: `internal/cli/ask.go:NewAskCmd` -> `internal/domain/contract/types.go:Request` via `internal/domain/jeq/compose.go:Compose`; value: `contract.Request`.
- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/infra/typesafeapi/client.go:Client.Evaluate` via `internal/cli/map.go:NewMapCmd`; value: `contract.Request`.
- Information flow: `internal/domain/pipeline/gate.go:Gate` -> `internal/cli/gate.go:NewGateCmd` via `internal/cli/gate.go:NewGateCmd`; value: `GateDecision`.

# Placement

Place a rule here when it is deterministic, reusable, and independent of an operating-system or wire mechanism. Keep document shape in contract, source/composition policy in jeq, and enrichment semantics in pipeline.
