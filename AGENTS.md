# Purpose

JEQ is a command-line client that turns TypeSafe System One judgments into safe, composable Unix data. It preserves request and response evidence as JSON and keeps policy decisions explicit and deterministic.

# Architecture

`cmd/jeq` is the production composition root. It wires the shell-facing [CLI](internal/cli/AGENTS.md) to infrastructure adapters for [source input and rendering](internal/infra/AGENTS.md) and the TypeSafe HTTP API. The CLI depends inward on the domain contracts, composition rules, and [pipeline](internal/domain/AGENTS.md); the domain does not depend on Cobra, HTTP, files, or renderers.

# Modules

- [Command](cmd/jeq/AGENTS.md): process entry point and dependency wiring.
- [Examples](AGENTS.md): executable workflow examples and user-facing recipes.
- [Internal](internal/AGENTS.md): application implementation and test-support boundaries.

# Landmarks

- `cmd/jeq/main.go:main`: builds production dependencies and starts the CLI.
- `internal/cli/run.go:RunWithDeps`: executes a fresh command tree and maps failures to process exits.
- `internal/domain/jeq/compose.go:Compose`: resolves native or composed request documents.
- `internal/domain/pipeline/pipeline.go:Enrich`: appends one judgment response to a JSON record.
- `internal/domain/pipeline/gate.go:Gate`: applies offline numeric policy to evidence.
- `internal/infra/typesafeapi/client.go:Client.Evaluate`: crosses the wire to TypeSafe System One.

# Boundary flows

- Information flow: `cmd/jeq/main.go:main` -> `internal/cli/run.go:RunWithDeps` via `cmd/jeq/main.go:main`; value: `cli.AskDeps`.
- Information flow: `internal/cli/ask.go:NewAskCmd` -> `internal/domain/jeq/compose.go:Compose` via `internal/cli/ask.go:NewAskCmd`; value: `jeq.ComposeInput`.
- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/infra/typesafeapi/client.go:Client.Evaluate` via `internal/cli/map.go:NewMapCmd`; value: `contract.Request`.
- Information flow: `internal/infra/typesafeapi/client.go:Client.Evaluate` -> `internal/infra/render/json.go:JSON` via `internal/cli/ask.go:NewAskCmd`; value: `contract.Response`.
- Information flow: `internal/domain/pipeline/gate.go:Gate` -> `internal/infra/render/json.go:JSON` via `internal/cli/gate.go:NewGateCmd`; value: `[]byte`.

# Placement

Put new command behavior in the CLI, request/response semantics in the domain, and operating-system or network concerns in infrastructure. Add a new top-level module only when a cohesive capability has its own ownership boundary and dependency direction; keep small implementation groupings under their owning guide.
