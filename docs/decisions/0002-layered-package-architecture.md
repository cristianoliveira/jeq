# 0002. Layered package architecture

- Status: Accepted
- Date: 2026-09-19

## Context

`gev` wraps external dependencies (Cobra, `net/http`, filesystem/stdin) around a stable center: the TypeSafe System One contract and gev's own input rules. We want an Onion-style dependency rule — external concerns point inward — but proportionate to a CLI.

An application/use-case layer was rejected during review: in a CLI, the command is the use case. A separate layer would only relay flags to ports.

## Decision

### Three rings plus a composition root

```text
cmd/gev → main            wiring only: builds adapters, injects into cli
internal/cli              shell: Cobra commands, flags, exit mapping
internal/infra            adapters: each wraps exactly one external dependency
internal/domain           center: pure Go, standard library only
```

### Folder structure

```text
internal/
├── domain/
│   ├── contract/          TypeSafe language: request, response, decode, validate
│   └── gev/               gev rules: ask modes, source matrix, compose, error codes, formats
├── infra/
│   ├── typesafeapi/       net/http client, bearer auth, bounded retries
│   ├── render/            deterministic JSON writer
│   └── source/            explicit file/stdin readers
└── cli/
    ├── root.go            home view
    ├── ask.go, models.go, validate.go
    └── exit.go            error code → exit class
```

### Dependency rules

```text
domain → standard library only
infra  → domain
cli    → domain
main   → cli + infra
```

No package imports `cmd/gev`. Infra never imports cli. Cycles are forbidden.

### Ports live with consumers

`cli` declares the small interfaces it needs (`Evaluator`, `ModelsLister`, `EncodeFunc`, source readers). `infra` satisfies them implicitly. `main` performs all construction and injection. No package exists solely to hold interfaces.

### Command handler rule

A command handler may only parse → call → map. Decisions (mode conflicts, validation, retry policy, source precedence) belong to `domain` so they are testable without Cobra. Handler logic beyond the rule is a review failure.

### External dependency containment

| Dependency | Only importer |
|---|---|
| Cobra | `internal/cli` |
| `net/http`, retry logic | `internal/infra/typesafeapi` |
| os/fs and stdin | `internal/infra/source` |
| `os.Getenv` | `cmd/gev`, `internal/cli` |

### Enforcement

`depguard` rules plus an import-architecture test run in the normal gate (`make check`), so a violated arrow fails the build rather than code review.

### Testing

- `domain`: table-driven unit tests, no I/O.
- `infra/typesafeapi`: real client against `httptest.Server`; no transport mocks.
- `infra/render`: deterministic JSON golden files and semantic round trips.
- `cli`: end-to-end through a fresh command tree per execution with injected streams.
- `internal/fixtures`: sanitized shared request/response fixtures.

## Consequences

- The TypeSafe contract and gev rules evolve and test with no Cobra or HTTP present.
- Each external dependency has one escape hatch; replacing the HTTP layer touches one package.
- Ports add minor indirection; they are justified by the infra boundary and wiring, not mock ceremony.
- Handler bloat is the main drift risk; the parse → call → map rule and lint enforcement guard it.
