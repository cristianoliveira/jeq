---
id: TASK-0049
title: State self-discoverable CLI contract in README
status: done
depends_on: []
priority: normal
tags: [docs, cli]
---

# State self-discoverable CLI contract in README

## Problem
jeq's README does not explicitly state that users and agents can discover the complete CLI through native help and built-in examples without installing a jeq-specific skill.

## Desired outcome
Users and agents understand that jeq teaches its complete command and workflow surface through the executable itself; no jeq-specific agent skill is required.

## Acceptance criteria
- [ ] README explicitly states the self-discoverability contract.
- [ ] README points to `jeq --help`, `jeq <command> --help`, and `jeq examples` as the discovery path.
- [ ] Wording does not imply that external documentation or general shell knowledge is unnecessary.
- [ ] Documentation checks pass.

## Non-goals
- Do not add or publish an agent skill.
- Do not change CLI behavior.

