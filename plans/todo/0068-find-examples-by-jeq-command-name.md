---
id: TASK-0068
title: Find examples by jeq command name
status: todo
depends_on: []
priority: high
tags: [cli, discovery, examples, agents, usability]
---

# Find examples by jeq command name

## Problem
Agents reasonably try `jeq examples map`, `jeq examples rank`, and other command names to verify usage before spending an API request, but examples are currently addressed only by workflow IDs such as `map-gate` and `rank-top-k`. The obvious discovery path fails.

## Desired outcome
Every judgment and offline policy command has an obvious example lookup by its own command name. An agent can inspect the canonical workflow, cost, privacy, input shape, and output shape before deciding whether to run it.

## Acceptance criteria
- [ ] `jeq examples ask|map|rate|rank|reduce|gate|validate` each exits successfully and shows a canonical existing recipe without making a network request.
- [ ] Preserve every existing workflow recipe ID and its current output.
- [ ] Command-name lookups reuse recipe data rather than copying shell snippets into another source of truth.
- [ ] When several recipes cover one command, show the simplest canonical recipe first and name the other related recipe IDs.
- [ ] `jeq examples` explains that users can look up examples by command name as well as workflow name.
- [ ] The help for `ask`, `map`, `rate`, `rank`, `reduce`, `gate`, and `validate` points to its exact `jeq examples <command>` lookup.
- [ ] Tests exercise every command-name lookup, unknown names, existing workflow names, and zero provider calls.
- [ ] Fresh watcher passes at clean committed HEAD.

## Non-goals
- Executing example pipelines.
- Adding paid demo requests.
- Replacing workflow recipes with command-only snippets.

## Constraints
- Example discovery is offline.
- Keep one source of truth for recipe content and network-cost statements.

