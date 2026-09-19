---
id: TASK-0005
title: Ask modes and composition
status: done
depends_on: [TASK-0004]
priority: high
tags: []
---

# Ask modes and composition

## Problem
Native vs composed mode with exactly one state source is the core jeq rule; conflicts must fail before any I/O.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Native mode accepts exactly `--request`; any composed-source flag with it fails pre-I/O with JEQ_SOURCE_CONFLICT.
- [ ] Composed mode requires `--questions` plus exactly one of `--state`, `--state-file`, or `--state-json`; missing or conflicting sources fail pre-I/O with stable codes.
- [ ] `Compose` is pure: it takes resolved bytes/model, performs no file/env/network I/O, and preserves unknown fields.
- [ ] Text state encodes as a JSON string; JSON state accepts object/array/string and rejects scalar number/bool/null.
- [ ] Model precedence resolves only in cli: `--model` > `TYPESAFE_DEFAULT_MODEL` > `jev-latest`; domain receives an explicit model.
- [ ] Full conflict matrix and fake-server zero-request tests cover happy and unhappy paths.

## Notes
Delivered from ADR 0001 and QA checks D1-8..D1-13; original generated criteria were placeholders.

