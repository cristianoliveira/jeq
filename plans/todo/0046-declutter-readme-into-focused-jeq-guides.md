---
id: TASK-0046
title: Declutter README into focused JEQ guides
status: todo
depends_on: []
priority: high
tags: [documentation, nix, onboarding]
---

# Declutter README into focused JEQ guides

## Problem
The flake is installable, but README only shows checkout-local commands and carries installation, onboarding, composition, command reference, configuration, output, safety, and development detail in one 288-line page. New users cannot quickly distinguish the shortest install path from deeper reference material.

## Desired outcome
A new reader can install the published JEQ release from its remote flake, understand the product, and run a first judgment from a short README. Detailed workflows remain easy to find in small, purpose-specific guides.

## Context
The flake already exposes default/named package and app outputs. Verified remote command:

```sh
nix run github:cristianoliveira/jeq/v0.1.0-rc.1 -- version
```

The current README is 288 lines with ten conceptual sections. Preserve its useful content but give each reader one obvious path rather than deleting important contracts.

## Acceptance criteria
- [ ] README starts with product purpose, a remote pinned-release Nix install/run path, one representative pipeline, core boundaries, and links to focused guides.
- [ ] Installation guide distinguishes pinned release install/run, current-main flake use, downloadable GitHub artifacts, Go install, and local checkout development; commands are executable and do not imply `.#jeq` works outside a checkout.
- [ ] Getting-started guide covers credential handling, native request creation, offline validation, first paid ask, result shape, and cost/privacy warning.
- [ ] Composition guide preserves the `jq`/JEQ/shell ownership model, primitive cost shapes, input modes, pipeline safety, and links to executable examples.
- [ ] CLI reference guide preserves command purposes, network behavior, output formats, configuration/model precedence, exit codes, trace behavior, and safety boundaries.
- [ ] README is materially shorter and does not duplicate whole guide sections.
- [ ] All relative links resolve, all documented commands and command names match current CLI behavior, and Nix remote run plus checkout install examples are verified.
- [ ] Existing architecture, development, release, and examples documentation remains linked and is not needlessly rewritten.
- [ ] Documentation changes do not change CLI, package, or release behavior.

## Constraints and non-goals
- Do not change the flake merely to rewrite documentation; its remote package/app outputs already work.
- Keep the published `v0.1.0-rc.1` command pinned and distinguish it from moving `main`.
- Do not hide that network judgments require a TypeSafe credential and spend account budget.
- Do not add generated docs or duplicate Cobra help exhaustively.

