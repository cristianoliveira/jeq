---
id: TASK-0035
title: Explain JEQ product goal and first workflow
status: done
depends_on: []
priority: high
tags: []
---

# Explain JEQ product goal and first workflow

## Problem
The repository has no root README, so a new user cannot understand why JEQ exists, whether it fits their problem, or how to reach a safe first result.

## Desired outcome
A reader understands JEQ's goal in the first screen, can decide whether it fits, and can validate then run one request without reading internal decisions.

## Acceptance criteria
- [x] Root `README.md` leads with the approved goal: make AI judgment a safe, composable Unix primitive.
- [x] Explain the problem, intended users, evidence-not-action boundary, and ask/map/reduce/gate mental model in plain language.
- [x] Include accurate local install, credential, offline validation, first request, examples discovery, configuration, output/exit, privacy, and development guidance.
- [x] Use current built CLI syntax and JEQ names only. Do not claim a hosted install path, package release, license, or behavior not evidenced by the repository.
- [x] Link detailed examples, architecture, development guide, and decisions instead of duplicating them.
- [x] Review every copyable command against current help; run documentation checks/full gate; commit and close the task.

## Non-goals
No badges, marketing claims, screenshots, roadmap, API reference, contribution policy, or release instructions without an established project contract.

