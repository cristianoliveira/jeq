---
id: TASK-0050
title: Standardize product name as lowercase jeq
status: doing
depends_on: []
priority: high
tags: [branding, docs]
---

# Standardize product name as lowercase jeq

## Problem
Repository prose still renders the product name as uppercase JEQ. Cristian established lowercase jeq as the only correct product spelling.

## Desired outcome
The product has one written identity everywhere: `jeq`, including at sentence starts and in headings. Uppercase remains valid only inside established configuration identifiers such as `JEQ_CONFIG`, `JEQ_TRACE_ID`, and `JEQ_BIN`.

## Acceptance criteria
- [ ] Current user-facing documentation, CLI prose, code comments, module guidance, tests, release notes, examples, and tracked task records use lowercase `jeq` as the product name.
- [ ] Root README title, logo alternative text, and SVG accessible titles use lowercase `jeq`.
- [ ] Environment variables and identifiers containing the uppercase `JEQ_` namespace remain unchanged.
- [ ] Root AGENTS.md records lowercase `jeq` as the durable naming rule.
- [ ] A case-sensitive tracked-file audit finds no standalone uppercase `JEQ` product spelling.
- [ ] Normal checks pass without adding a grep-based automated test.

## Non-goals
- Do not rename the executable, module path, repository, `_jeq` evidence key, or `JEQ_*` environment variables.
- Do not rewrite Git history.

