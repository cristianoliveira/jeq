---
id: TASK-0051
title: Use plain README language for CLI discovery and cautions
status: todo
depends_on: []
priority: normal
tags: [docs, readability]
---

# Use plain README language for CLI discovery and cautions

## Problem
README uses abstract terms such as command surface and boundaries. Cristian wants direct language that does not sound LLM-generated.

## Desired outcome
README explains command discovery and operating cautions in direct, concrete language.

## Acceptance criteria
- [ ] Replace “command surface” with the action a user takes and sees.
- [ ] Replace the “Boundaries” heading with a plain heading that matches its practical cautions.
- [ ] Preserve the self-discovery claim and all existing safety, cost, exit, and ownership facts.
- [ ] Documentation checks pass.

## Non-goals
- Do not change CLI behavior or terminology outside README.

