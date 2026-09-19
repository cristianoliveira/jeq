---
id: TASK-0025
title: Probabilistic reduce and model configuration
status: todo
depends_on: [TASK-0024]
priority: normal
tags: []
---

# Probabilistic reduce and model configuration

## Problem
`gev map` can judge each record independently, but callers cannot ask one bounded probabilistic question about a complete collection without manually reshaping it into an envelope. Normal composed commands also need a durable default model without repeating `--model` in every invocation.

## Desired outcome
The existing CLI has a small map/reduce/gate algebra. `gev reduce` turns bounded JSON-array or NDJSON input into one collection envelope and performs exactly one judgment. Map and reduce resolve their model from transparent configuration by default.

## Acceptance criteria
- [ ] ADR 0005 defines `reduce` as a one-call probabilistic aggregate, not an associative or iterative fold. A true N-call evolving fold and any embedded jq/query language remain deferred.
- [ ] `gev reduce --as <name> (--questions <file> | --questions-json <document>) [--input json|ndjson] [--model <override>]` accepts a JSON array or bounded NDJSON values, preserves order and raw JSON semantics, and produces one JSON envelope: `{"items":[...],"_gev":{"<name>":<complete response>}}`.
- [ ] Reduce sends exactly one request whose state is the complete `items` array. It reuses strict question/request validation and the append-only TASK-0023 engine; it never interprets answers or performs actions.
- [ ] `--questions-json` accepts the exact same strict, bounded document as `--questions`, without reading stdin or creating a temporary file. Map and reduce support it consistently; selecting file and inline sources together is a local source-conflict error.
- [ ] Empty collections, malformed values, recursive duplicate keys, excess records/bytes, `_gev` collisions, invalid flags/questions, and unavailable model configuration fail locally before client construction. API/auth/timeout/interruption preserve 1/1/1/130 behavior.
- [ ] Output is one JSON document even for NDJSON input. Help states collection limits, one-request cost, complete-input privacy, and the difference between aggregate and dependent fold.
- [ ] The resolved model follows explicit precedence: `--model`, `TYPESAFE_DEFAULT_MODEL`, user or explicitly selected config, then `jev-latest`. Normal map/reduce examples omit `--model`; every response still records the resolved model.
- [ ] Configuration is bounded strict JSON and initially permits only `default_model`. It never stores credentials or API endpoints. Automatic lookup uses user config only; repository config requires an explicit path so an untrusted checkout cannot silently redirect or alter network policy.
- [ ] Missing config is not an error; malformed, duplicate, unknown, oversized, or wrong-typed configuration fails before network with actionable recovery. Discovery exposes the resolved default model and its source without exposing secrets.
- [ ] Tests cover JSON/NDJSON order, scalar/object/array items, unsafe numbers/Unicode, one exact request, map-to-reduce-to-gate composition, empty/malformed/oversized collections, model precedence/config failures, response extras, and operational propagation.
- [ ] Support or incident examples include one readable aggregate decision and explicit projection before logging. Full checks, govulncheck, and an opt-in fake/live cost receipt pass.

## Non-goals
- No embedded jq, `gev eval`, traversal language, arbitrary expressions, iterative probabilistic fold, hidden batching, concurrency, actions, or model-selected paths.
- No project configuration auto-discovery, credentials, base URLs, retry policy, or other network settings in the first config schema.

## Notes
External `jq` remains an optional Unix transformer between `map`, `reduce`, and `gate`. Revisit a programmable evaluator only after real pipelines show that these primitives are insufficient.
