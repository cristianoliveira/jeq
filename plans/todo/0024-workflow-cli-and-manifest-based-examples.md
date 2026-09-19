---
id: TASK-0024
title: Workflow CLI and manifest-based examples
status: todo
depends_on: [TASK-0023]
priority: high
tags: []
---

# Workflow CLI and manifest-based examples

## Problem
Daily gev users need one readable subcommand for multi-stage decisions. The workflow engine must be exposed through an agent-safe CLI, and temporary Python wrappers must be replaced by declarative manifests with equivalent end-to-end evidence.

## Desired outcome
Daily users can run and inspect complex decisions with one gev command and ordinary stdin/stdout, without Bash orchestration or a Python subprocess adapter.

## Acceptance criteria
- [ ] `gev workflow validate --file <manifest>` is fully offline, validates the manifest and every referenced local document, and emits one JSON receipt; invalid workflows exit 2 before client construction.
- [ ] `gev workflow run --file <manifest>` reads declared text or JSON state from stdin (or one explicit state source if justified), executes through existing client/renderer ports, and emits one generic workflow receipt.
- [ ] Help is concise and copyable; unknown flags/versions/operators include precise recovery. No prompts, shell execution, HTTP references, or hidden ambient state.
- [ ] Manifest and referenced files use bounded reads; relative paths are cleaned, symlinks resolved, and required to remain below the resolved manifest directory.
- [ ] Compiled-binary black-box tests cover validate/run, text/JSON input, zero/one/two requests, 0/10/11 decisions, 1/2/130 propagation, malformed manifests/state, path traversal/symlink escape, candidate allowlisting, exact request bodies, newline/stderr/redaction, and interruption.
- [ ] Replace support/release/incident Python programs and adapter with equivalent versioned workflow manifests; remove Python from the dev shell if no longer needed. Keep readable diagrams, fixtures, questions/catalogs, and copy-paste one-command examples.
- [ ] Existing Bash examples remain explicitly low-level; manifest workflows become the recommended daily path.
- [ ] Full normal gate, govulncheck, architecture rules, and an opt-in live run of each synthetic manifest pass; record calls/tokens/cost without raw state or credentials.

## Non-goals
- No arbitrary action execution, DAGs/loops/concurrency, remote includes, output templates, mutation, persistent state, or general-purpose expression language.

## Notes
The CLI is a decision engine, not an automation engine. Callers remain responsible for any side effect after inspecting decision and exit status.

