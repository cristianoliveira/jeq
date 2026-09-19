---
id: TASK-0012
title: models, version, and home view
status: todo
depends_on: [TASK-0002, TASK-0006]
priority: normal
tags: []
---

# models, version, and home view

## Problem
Agents need discovery and readiness checks without network access or credentials.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] `gev models` calls the real models client with normal auth/timeout/base-url classification and renders the server's alias list without assuming versioned IDs are listed.
- [ ] `gev version` emits injected build name/version/commit deterministically; release values come from linker flags, development values remain explicit.
- [ ] No-argument home view is offline and structured: identity, purpose, credential-ready boolean (never value), resolved default model, valid commands, and one actionable next step.
- [ ] Home/version never open network or read stdin; home checks only whether the credential env value is non-empty.
- [ ] Every command honors selected renderer once TOON lands; exactly one document/newline and stable error exits.
- [ ] Tests cover credential present/absent, model env override, live-shape model fixture (aliases only), linker-value injection, and zero network/read counts.

## Notes
Only `models` requires credentials/network. QA live acceptance is TASK-0019.

