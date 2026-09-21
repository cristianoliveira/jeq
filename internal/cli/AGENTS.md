# Purpose

The CLI owns Cobra commands, flag and source policy, human diagnostics, stream orchestration, and process-exit mapping.

# Boundaries

It translates shell input into domain calls and consumes injected ports. Provider selection and the shared model resolver belong here; they produce one explicit connection profile for all network commands. It does not implement JSON contract semantics, HTTP, filesystem access, or rendering details.

# Connections

- [Domain](../domain/AGENTS.md): provides request contracts, source composition, and map/reduce/gate operations consumed by commands.
- [Infrastructure](../infra/AGENTS.md): supplies injected source, renderer, and TypeSafe client implementations through the composition root.

# Landmarks

- `internal/cli/root.go:NewRootCmd`: assembles the available Cobra command tree.
- `internal/cli/run.go:RunWithDeps`: executes commands and maps errors to exit classes.
- `internal/cli/ask.go:NewAskCmd`: constructs the ask workflow command.
- `internal/cli/map.go:NewMapCmd`: constructs the bounded per-record enrichment command.
- `internal/cli/gate.go:NewGateCmd`: constructs the offline policy command.

# Boundary flows

- Information flow: `internal/cli/ask.go:NewAskCmd` -> `internal/domain/jeq/compose.go:Compose` via `internal/cli/ask.go:NewAskCmd`; value: `jeq.ComposeInput`.
- Information flow: `internal/domain/pipeline/pipeline.go:Enrich` -> `internal/cli/map.go:NewMapCmd` via `internal/cli/map.go:NewMapCmd`; value: `[]byte`.
- Information flow: `internal/domain/pipeline/gate.go:Gate` -> `internal/cli/gate.go:NewGateCmd` via `internal/cli/gate.go:NewGateCmd`; value: `pipeline.GateDecision`.

# Placement

Put new user-facing actions and shell policy here. Put reusable semantics in domain and concrete I/O in infrastructure; add a separate command module only when a command family owns a distinct workflow boundary.
