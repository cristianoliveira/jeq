---
id: TASK-0008
title: JSON renderer
status: done
depends_on: [TASK-0003]
priority: normal
tags: []
---

# JSON renderer

## Problem
JSON is the compatibility output contract and must be deterministic and lossless.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Render success and error values as exactly one deterministic JSON document plus one trailing newline.
- [ ] Preserve resolved model, every answer/type, probabilities, confidence, score legends, token usage, and unknown fields without semantic loss.
- [ ] Error document exposes only code/message/recovery; never internal cause, secret, stack, or raw dependency prose.
- [ ] Renderer writes to an injected `io.Writer`; cli depends on a renderer interface and does not import infra/render (ADR 0002).
- [ ] Golden fixtures cover full success, every primitive, unknown fields, stable error shape, escaping, empty optional fields, and redaction.
- [ ] JSON output parses and round-trips to a semantically identical contract value.

## Notes
JSON is the D2 interim default. TASK-0009 adds TOON and flips the pre-v1 default in D3.

