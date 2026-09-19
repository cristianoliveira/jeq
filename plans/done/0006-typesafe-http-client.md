---
id: TASK-0006
title: TypeSafe HTTP client
status: done
depends_on: [TASK-0002, TASK-0003]
priority: normal
tags: []
---

# TypeSafe HTTP client

## Problem
jeq needs the two endpoints with bearer auth, timeouts, body bounds, and status-to-error classification.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Inject base URL, `http.Client`, API key, clock/sleeper hooks as needed; no globals and no client construction in domain.
- [ ] Implement real contract paths: `POST /v1/systemone` and `GET /v1/models`; bearer auth, JSON content type, and no credential in errors/logs.
- [ ] Missing key fails pre-network with JEQ_AUTH_MISSING. Map 401→JEQ_AUTH_REJECTED; server 422→new JEQ_REQUEST_REJECTED (exit 1); 429/529→JEQ_RATE_LIMITED; other 5xx→JEQ_SERVER_ERROR; transport→JEQ_NETWORK_ERROR; deadline→JEQ_TIMEOUT; malformed/oversize reply→JEQ_RESPONSE_INVALID.
- [ ] Response bodies are bounded (8 MiB normal; 64 KiB error-detail read) and always closed; server detail is sanitized before recovery output.
- [ ] Tolerantly decode success responses while retaining unknown fields and all usage/model data.
- [ ] `httptest.Server` covers both endpoints, auth header, every status class, malformed/oversize bodies, timeout, connection failure, and secret redaction; no real network in normal tests.

## Notes
Retry scheduling belongs to TASK-0007. `JEQ_REQUEST_INVALID` stays local/exit 2; server 422 uses `JEQ_REQUEST_REJECTED`/exit 1.

