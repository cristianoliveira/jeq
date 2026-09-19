---
id: TASK-0002
title: Stable error codes and exit mapping
status: done
depends_on: [TASK-0001]
priority: high
tags: []
---

# Stable error codes and exit mapping

## Problem
Scripts and agents need stable failure classes; matching error prose breaks automation.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Code format locked: `JEQ_<AREA>_<REASON>`, uppercase A–Z/0–9/underscore (OQ-7 resolved).
- [ ] Initial registry: JEQ_AUTH_MISSING, JEQ_AUTH_REJECTED, JEQ_REQUEST_INVALID, JEQ_SOURCE_CONFLICT, JEQ_INPUT_INVALID, JEQ_RATE_LIMITED, JEQ_SERVER_ERROR, JEQ_RESPONSE_INVALID, JEQ_NETWORK_ERROR, JEQ_TIMEOUT, JEQ_INTERRUPTED; one internal cause per code, wrapped with `%w`.
- [ ] Golden snapshot test fails on any registry add/remove/rename (`contract/error_codes.golden`).
- [ ] Exit mapping is a pure total function: every code → exactly one of 0/1/2/130; table-driven.
- [ ] Error document shape: selected format on stdout, one document + trailing newline, fields code/message/recovery only; cause never serialized.
- [ ] Redaction test: no API key, stack trace, or raw dependency text in stdout/stderr.

## Notes
Resolves Kelly OQ-7. Confidence/probability values never influence exit class (ADR 0001).

