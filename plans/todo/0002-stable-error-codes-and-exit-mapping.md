---
id: TASK-0002
title: Stable error codes and exit mapping
status: doing
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
- [ ] Code format locked: `GEV_<AREA>_<REASON>`, uppercase A–Z/0–9/underscore (OQ-7 resolved).
- [ ] Initial registry: GEV_AUTH_MISSING, GEV_AUTH_REJECTED, GEV_REQUEST_INVALID, GEV_SOURCE_CONFLICT, GEV_INPUT_INVALID, GEV_RATE_LIMITED, GEV_SERVER_ERROR, GEV_RESPONSE_INVALID, GEV_NETWORK_ERROR, GEV_TIMEOUT, GEV_INTERRUPTED; one internal cause per code, wrapped with `%w`.
- [ ] Golden snapshot test fails on any registry add/remove/rename (`contract/error_codes.golden`).
- [ ] Exit mapping is a pure total function: every code → exactly one of 0/1/2/130; table-driven.
- [ ] Error document shape: selected format on stdout, one document + trailing newline, fields code/message/recovery only; cause never serialized.
- [ ] Redaction test: no API key, stack trace, or raw dependency text in stdout/stderr.

## Notes
Resolves Kelly OQ-7. Confidence/probability values never influence exit class (ADR 0001).

