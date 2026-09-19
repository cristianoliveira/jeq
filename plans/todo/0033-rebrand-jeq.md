---
id: TASK-0033
title: Rebrand JEQ to JEQ
status: doing
depends_on: []
priority: high
tags: []
---

# Rebrand JEQ to JEQ

## Problem
The product name JEQ no longer matches the desired identity. Replace the executable, public contracts, configuration namespace, module path, and repository-facing language with JEQ as one intentional breaking release.

## Desired outcome
One product name exists everywhere: JEQ. A fresh checkout builds and installs `jeq`; its public data/config contracts use the JEQ namespace; no supported alias or compatibility layer keeps JEQ alive.

## Acceptance criteria
- [ ] Rename executable/build target/command package from `jeq` to `jeq`; `jeq`, `jeq --help`, completion, examples, development commands, Nix install/checks, distribution artifacts, and shell scripts use the new name. Do not ship a `jeq` alias.
- [ ] Rename the Go module/import root to `github.com/cristianoliveira/jeq`, `cmd/jeq` to `cmd/jeq`, and `internal/domain/jeq` package/path to `internal/domain/jeq`. Update architecture import guards and all Go identifiers/comments whose name encodes the product.
- [ ] Rename stable error values from `JEQ_*` to `JEQ_*`; update fixtures, manifests, CLI error assertions, docs, and security/redaction tests. Exit meanings remain unchanged.
- [ ] Rename composable evidence envelope `_jeq` to `_jeq` everywhere, including map/reduce/gate logic, pointers, examples, fixtures, collision errors, privacy projections, tests, and ADRs. Preserve all other envelope semantics.
- [ ] Rename product-owned environment/shell variables (`JEQ_BIN`, `JEQ_MODEL`, `JEQ_BASE_URL`, etc.) and config locations from `jeq` to `jeq` (`~/.config/jeq/config.json`). Keep vendor-owned `TYPESAFE_*` variables unchanged.
- [ ] Rename tracked file/directory names and current repository-facing prose from JEQ/jeq to JEQ/jeq, including QA filenames and completed-plan filenames. Task IDs and Git history remain stable. Local checkout directory name is outside the code change.
- [ ] Treat this as an intentional breaking release: no old executable, error-code alias, `_jeq` fallback, dual config lookup, deprecated environment variable, or import compatibility package.
- [ ] Finish TASK-0032 only after the rename, so every Cobra command/help/example is authored directly as JEQ. Bare `jeq` and `jeq examples` use native Cobra help per their tasks.
- [ ] Tests first at each boundary: binary existence/name, module/import graph, codes, envelope writes/reads/collisions, config precedence/path, example execution, CLI help/errors, no old alias, and absence scan.
- [ ] Final tracked-tree scan (`git ls-files` plus content search, case-insensitive) finds no `jeq` in paths or content. Document any technically unavoidable generated/tool cache exception; ignored local files do not count and must not be committed.
- [ ] Update decisions/architecture/development/QA, run full Nix checks, govulncheck, built-binary black-box and fake-endpoint workflows, architecture review, independent QA, then commit.

## External repository boundary
Changing a hosted GitHub repository name is an external operation and is not assumed by the local code migration. The module path is intentionally prepared for `github.com/cristianoliveira/jeq`; record whether a remote exists and any remaining hosting action.

