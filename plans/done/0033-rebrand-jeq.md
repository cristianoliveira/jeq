---
id: TASK-0033
title: Complete full jeq rebrand
status: done
depends_on: []
priority: high
tags: []
---

# Complete full jeq rebrand

## Problem
The predecessor product name no longer matches the desired identity. Replace its executable, public contracts, configuration namespace, module path, and repository-facing language with jeq as one intentional breaking release.

## Desired outcome
One product name exists everywhere: jeq. A fresh checkout builds and installs `jeq`; public data and configuration contracts use the jeq namespace; no supported alias or compatibility layer retains the predecessor identity.

## Acceptance criteria
- [x] The executable, build target, command package, help, completion, examples, development commands, Nix installation, distribution artifacts, and shell scripts use `jeq`. No predecessor executable alias is shipped.
- [x] The Go module is `github.com/cristianoliveira/jeq`; command and domain paths use `cmd/jeq` and `internal/domain/jeq`. Architecture guards, imports, identifiers, and comments use the new identity.
- [x] Stable error values use `JEQ_*`; fixtures, manifests, CLI assertions, documentation, and redaction tests agree. Exit meanings remain unchanged.
- [x] Composable evidence uses `_jeq` everywhere: map/reduce/gate logic, pointers, examples, fixtures, collisions, privacy projections, tests, and decisions. Envelope semantics otherwise remain unchanged.
- [x] Product-owned shell variables use `JEQ_*`; configuration resolves only `${XDG_CONFIG_HOME:-$HOME/.config}/jeq/config.json`. Vendor-owned `TYPESAFE_*` variables remain unchanged.
- [x] Tracked path names and current repository-facing prose use jeq, including QA and completed-plan filenames. Task IDs and Git history remain stable. The local checkout directory is outside the code change.
- [x] This is an intentional breaking release: no predecessor executable, error alias, evidence fallback, dual config lookup, deprecated environment variable, or import compatibility package.
- [x] TASK-0032 native Cobra discovery works directly under jeq. Bare `jeq` and `jeq examples` use native Cobra help.
- [x] Tests cover binary presence and predecessor absence, module/import graph, codes, envelope writes/reads/collisions, config precedence/path, example execution, CLI help/errors, and tracked-tree absence scans.
- [x] Final tracked-tree path/content scans find no standalone predecessor product token, legacy code prefix, or legacy envelope key. Unrelated identifiers containing the same three-letter substring and ignored local caches do not count.
- [x] Decisions, architecture, development guidance, and QA are current; full Nix checks, govulncheck, built-binary black-box tests, fake-endpoint workflows, architecture review, and independent QA pass.

## External repository boundary
Changing a hosted GitHub repository name is an external operation and is not assumed by the local code migration. The module path is prepared for `github.com/cristianoliveira/jeq`. No Git remote is configured in this checkout, so no hosting action was available.
