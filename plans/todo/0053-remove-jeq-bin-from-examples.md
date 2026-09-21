---
id: TASK-0053
title: Remove JEQ_BIN from examples
status: doing
depends_on: []
priority: normal
tags: [cli, examples, documentation, usability, testing]
---

# Remove JEQ_BIN from examples

## Problem
User-facing examples expose `JEQ_BIN` as if it were required configuration, although it exists mainly as a test injection seam. Several snippets also look like script templates: they initialize binary variables, enable strict shell options, install traps, or export state before showing the actual jeq command. This obscures the normal contract—install `jeq` on `PATH` and run `jeq`—and makes simple examples harder and less safe to paste into an interactive terminal.

## Desired outcome
Every documented and CLI-generated example is a complete command or bounded command block that a user can paste directly into a terminal. Examples call `jeq` by name, state prerequisites before the block, include or pipe concrete input, and do not require users to create a wrapper script or understand a test-only binary override.

Executable recipe files may remain as reusable artifacts, but documentation must not require users to reproduce them. Those scripts also resolve `jeq` and `jq` normally through `PATH` rather than exposing binary-injection variables.

## Acceptance criteria

### User-facing examples
- [ ] `jeq examples <recipe>` outputs terminal-ready commands that invoke `jeq` directly; no output contains `JEQ_BIN`, `JQ_BIN`, `"$JEQ_BIN"`, or binary-variable initialization.
- [ ] Repository READMEs and guides show direct `jeq ...` commands or direct invocation of a checked-in executable recipe. They never ask users to create a script before trying an example.
- [ ] Each standalone snippet supplies concrete input inline or names an existing checked-in fixture from an explicit working directory. It contains no unexplained placeholders such as `input.json`, `path/to/file`, or an assumed temporary script.
- [ ] Required tools, credentials, provider selection, expected network calls, privacy boundary, and significant exit statuses remain stated outside the command block.
- [ ] Offline examples such as native validation remain runnable without credentials or network access.

### Safe terminal behavior
- [ ] Copy-pasting a snippet does not permanently mutate the caller's shell with `set -e`, `set -u`, `set -o pipefail`, `trap`, `cd`, or exported example-only variables.
- [ ] Where pipeline exit preservation matters, the example uses a bounded subshell or another terminal-safe form so `pipefail` cannot leak into the caller's session.
- [ ] Temporary files are avoided where a direct pipeline is readable. If one is necessary, creation and cleanup are contained in the pasted command block.
- [ ] Examples continue to quote data and paths safely and never evaluate model output as shell code.

### Executable recipes and tests
- [ ] Checked-in `examples/**/*.sh` scripts call `jeq` directly and keep their existing stdout, stderr, exit-status, privacy, retry, and request-count behavior.
- [ ] Test injection moves behind normal command lookup: tests place a fake or built `jeq` executable in a temporary directory and prepend that directory to `PATH` rather than teaching production examples about `JEQ_BIN`.
- [ ] Apply the same PATH-based seam to `jq` where removing `JQ_BIN` is necessary to keep example scripts free of binary-variable boilerplate.
- [ ] Tests execute representative exact snippets or recipe commands as behavior. Do not add grep/regex tests that only assert text absence.
- [ ] Existing online examples remain testable with deterministic fake endpoints and credentials; no paid or live API request is required.
- [ ] The configured watcher final gate passes.

## Scope
- `internal/cli/examples.go` and its behavioral tests.
- Current example READMEs and executable recipes under `examples/`.
- Guides that currently teach `JEQ_BIN`, especially `docs/guides/reduce.md`.
- Test helpers in `examples/examples_test.go` and related example suites.

## Non-goals
- Removing legitimate jeq configuration variables such as `JEQ_CONFIG`, `JEQ_PROVIDER`, `JEQ_DEFAULT_MODEL`, or `JEQ_TRACE_ID` when the example is specifically teaching that feature.
- Rewriting completed plan history that mentions the former test seam.
- Removing reusable checked-in recipe scripts when they provide value beyond the direct terminal example.
- Adding a CLI flag for selecting the jeq executable.

## Implementation sequence
1. Add or adapt behavior tests so a temporary `PATH` supplies fake `jeq`/`jq` commands without `JEQ_BIN` or `JQ_BIN`.
2. Simplify CLI-generated recipe blocks to direct, terminal-safe commands; execute representative blocks in tests.
3. Update checked-in shell recipes to call tools by name and preserve their observable contracts.
4. Replace README and guide snippets with direct commands or direct checked-in recipe invocations.
5. Audit current user-facing surfaces, run focused example tests, then run the watcher final gate.

## Notes
- `JEQ_BIN` currently appears in `internal/cli/examples.go`, multiple `examples/**/*.sh` and README files, `docs/guides/reduce.md`, and example test helpers.
- The variable was useful for test injection, but `PATH` is the standard shell seam and does not leak test architecture into product teaching.
- Historical mentions in `plans/done/` remain unchanged as project records.

