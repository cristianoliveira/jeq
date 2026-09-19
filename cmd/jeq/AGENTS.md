# Purpose

The command package owns the process entry point and production dependency wiring for JEQ.

# Boundaries

This package constructs infrastructure adapters, process streams, environment access, and the CLI dependency bundle. It does not own command behavior or domain rules.

# Connections

- [CLI](../../internal/cli/AGENTS.md): receives production dependencies and runs the command tree.
- [Infrastructure](../../internal/infra/AGENTS.md): provides source readers, JSON rendering, and the TypeSafe client that this package constructs.

# Landmarks

- `cmd/jeq/main.go:main`: constructs dependencies, invokes the CLI, and exits with its status.

# Boundary flows

- Information flow: `cmd/jeq/main.go:main` -> `internal/cli/run.go:RunWithDeps` via `cmd/jeq/main.go:main`; value: `cli.AskDeps`.

# Placement

Keep only executable bootstrap and composition-root wiring here. Put reusable command behavior in CLI and adapter behavior in infrastructure.
