# gev delivery plan

Board: `plans/todo/` and `plans/done/` (kanban: folder is truth, `TASK-NNNN` ids are stable).
Decisions: `docs/decisions/0001-agent-first-cli-contract.md`, `docs/decisions/0002-layered-package-architecture.md`.

## Deliveries

### D0 — Foundation

Tasks: TASK-0001, TASK-0002 · Owner: Dave

Deliverable: `gev version` runs from a real Go module; `make check` enforces gofmt, vet, golangci-lint (with depguard), tests with `-race -shuffle=on`, and the ADR-0002 import-architecture test. Stable error codes and the 0/1/2/130 exit mapping exist with table tests.

### D1 — Contract core

Tasks: TASK-0003, TASK-0004, TASK-0005 · Owner: Dave · QA: TASK-0016

Deliverable: pure `domain` compiles the full TypeSafe document model: strict JSON decode (duplicate keys rejected, unknown fields preserved), validation rules from ADR 0001, and ask-mode composition with the exactly-one-state-source rule. No I/O anywhere in the ring.

### D2 — Working ask

Tasks: TASK-0006, TASK-0007, TASK-0008, TASK-0010, TASK-0011 · Owner: Dave

Deliverable: `gev ask` works end to end against `httptest`: adapters for HTTP (auth, timeouts, bounded 429/529 retries with `Retry-After`), JSON rendering, and source readers. Exit codes and structured stdout/stderr hold.

### D3 — v1 contract parity

Tasks: TASK-0009, TASK-0012, TASK-0013 · Owner: Dave

Deliverable: TOON default output (or a recorded spike decision that changes ADR 0001), `gev models`, `gev validate` offline, and the no-argument home view. The full accepted v1 command surface exists.

### D4 — Release-ready v1

Tasks: TASK-0014, TASK-0015 · Owner: Dave · QA: verification gate

Deliverable: black-box binary suite (exit codes, stdout/stderr separation, trailing newline, SIGINT), govulncheck triaged, cross-build smoke, TOON round-trip suite, `docs/ARCHITECTURE.md` and `DEVELOPMENT.md`.

## Working agreements

- Code lands on `main` with commit messages referencing `TASK-NNNN`; the board moves via `kanban.py` commits immediately after each task closes.
- Every task is test-first; acceptance criteria live in the task file.
- Tools run inside `nix develop`; the normal gate is `make check` and prints `true` only on success.
- `.tmp/` stays untracked. No force operations, no rewriting history.
- Module path unless a remote says otherwise: `github.com/cristianoliveira/gev`.

## Sequencing decisions (QA open questions resolved)

- OQ-5: D2 ships JSON as the interim default output. TASK-0009 (D3) introduces TOON and flips the default. No external contract exists before the v1 tag (D4), so this is not a breaking change.
- OQ-6: `models`/`version`/home-view acceptance belongs to D3 (TASK-0012). D2 end-to-end acceptance covers `ask` only.
- OQ-4: binary-level timeout tests use the existing `--timeout` flag; no new knob.
