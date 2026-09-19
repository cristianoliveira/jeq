---
id: TASK-0029
title: Make CLI workflows self-discoverable
status: done
depends_on: [TASK-0028]
priority: high
tags: [axi, discovery]
---

# Make CLI workflows self-discoverable

## Problem
jeq exposes command names and flags, but installed agents cannot discover runnable examples, pipeline relationships, request cost, envelope shapes, privacy projections, or gate exits through the CLI. Repository examples are invisible unless an agent already knows to inspect the checkout.

## Desired outcome
A freshly installed agent can discover capabilities and reach a runnable, self-contained workflow without repository access: `jeq` → `jeq examples` → `jeq examples <id>`. Help remains local and concise, while detailed samples are progressive disclosure.

## Discoverability baseline
Current evidence: no-arg home is strong structured JSON, but only points to `jeq ask --help`; root/subcommand help lists flags without examples; no CLI command mentions repository samples, request counts, envelopes, privacy projection, map-vs-reduce semantics, or gate exits. Commands are discoverable; workflows are not. Baseline rating: installed CLI 4/10, source checkout 7/10.

## Acceptance criteria
- [x] Add an offline `jeq examples [id]` command. With no id it emits one compact JSON catalog; with one id it emits one detailed JSON recipe. It never needs credentials, config, filesystem reads, jq, or network access merely to discover content.
- [x] Catalog includes at least `ask-native`, `map-gate`, `reduce-gate`, and `map-reduce-gate`. Each compact item has stable id, one-sentence purpose, covered jeq commands, and explicit network-call formula/count. Catalog includes a copyable next step.
- [x] Each detailed recipe is self-contained and runnable from an installed binary using stdin plus inline/native JSON; it does not depend on repository question files or fixtures. Include purpose, covered commands, requirements, request cost, multiline shell, input/output shape, privacy note, exit semantics where relevant, and next step.
- [x] Recipe shell uses `JEQ_BIN=${JEQ_BIN:-jeq}`, safe quoting, `set -o pipefail` when gate is piped onward, no `eval`, no model-derived shell/path, and explicit source projection before log-safe output. Recipes may require jq only when composition needs it and must declare that requirement.
- [x] No-arg home adds `examples` to commands and changes its most useful next step to `run jeq examples`; remain compact structured JSON and preserve readiness/model fields.
- [x] Add concise Cobra `Example` blocks to `examples`, `ask`, `validate`, `map`, `reduce`, and `gate` help. Each has 2–3 relevant invocations or points to `jeq examples <id>`; required/default flags remain visible. Root help points to `jeq examples`.
- [x] Unknown example id returns structured `JEQ_INPUT_INVALID`, exit 2, no stderr, names the id, lists valid ids in recovery, and performs no dependency/network action. Extra args do the same. Help remains prose exit 0.
- [x] One source of truth owns catalog order/content and detailed recipes. Home command discovery and help references must not duplicate recipe bodies. Keep domain catalog independent from rendering; output through existing JSON renderer.
- [x] Tests first: catalog/detail JSON schema and deterministic ordering; every referenced id resolves; commands/coverage/request costs; no secrets/source checkout dependency; home path; local help examples; unknown/extra arg behavior; no network/dependency calls. Execute at least the self-contained validation recipe and fake-endpoint variants of network recipes where practical; do not make live model-quality assertions.
- [x] Measure discovery: fresh-agent route reaches the full Unix pipeline recipe in three commands or fewer and compact catalog output stays under 2 KiB. Record representative byte counts and round trips.
- [x] Update user-facing documentation to name `jeq examples` as the primary sample discovery path and repository examples as expanded executable references. Add QA/report, full checks, govulncheck, architecture/risk review, independent QA, and commits.

## Non-goals
- No interactive prompts, browser/docs opener, repository auto-discovery, copying/scaffolding files, dynamic remote catalog, TOON change, shell execution by jeq, model-generated recipes, telemetry, or change to map/reduce/gate semantics.

## Notes
This closes an AXI gap rather than adding another hidden README. Progressive disclosure is intentional: home identifies the next discovery command, catalog stays compact, detail carries the copyable recipe.

