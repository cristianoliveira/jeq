---
id: TASK-0041
title: Trace probabilistic CLI chains safely
status: todo
depends_on: [TASK-0042]
priority: high
tags: [cli, observability, privacy, composability]
---

# Trace probabilistic CLI chains safely

## Problem

JEQ preserves probabilistic evidence in stdout, but agents cannot observe how a
multi-record or multi-stage command resolved configuration, progressed through
evaluations and retries, or where a `map` / `rate` / `reduce` / `rank` / `gate`
chain failed. This makes failures in probabilistic workflows hard to localize
without reading source code or reproducing requests.

## Desired outcome

An agent can opt into a bounded, structured execution trace while keeping the
normal result stream unchanged:

```sh
export JEQ_TRACE_ID=review-42
jeq --verbose map ... 2>map.trace.ndjson |
  jeq --verbose rate ... 2>rate.trace.ndjson |
  jq ...
```

The result JSON/NDJSON remains the semantic evidence: what System One returned,
including probabilities, confidence, model, usage, and provider trace fields.
The verbose stderr trace explains how JEQ executed that chain: which operation
and record ran, which safe configuration was resolved, which request attempt or
retry occurred, and where a failure stopped progress.

This is not access to model chain-of-thought. JEQ observes explicit request
lifecycle and returned evidence only.

## Operational questions

The trace must let an agent answer:

1. Which JEQ command, probabilistic operation, and bounded record or collection
   was executing when the chain stopped?
2. Did failure occur during input validation, state selection, request creation,
   transport/retry, response validation, output writing, or offline policy?
3. Which model source, question names/types, input framing, pointer, timeout, and
   retry policy were active without revealing state or prompt content?
4. For a loop, how many records were seen, evaluated, emitted, failed, or
   suppressed from detailed tracing?
5. Across shell processes, which trace events belong to the same caller-defined
   chain?

Every emitted field must answer one of these questions.

## Ubiquitous language

- **Probabilistic chain:** explicit sequence of JEQ commands and returned
  evidence. It is not hidden model reasoning.
- **Execution trace:** structured lifecycle metadata written to stderr.
- **Evidence:** lossless System One response written to stdout under `_jeq` or
  as the command result.
- **Trace ID:** caller-provided identifier that correlates separate JEQ
  processes in one shell workflow.
- **Operation index:** one-based bounded position of a record or collection
  evaluation inside one command.

## Acceptance criteria

### Opt-in CLI contract

- [ ] A global, inherited, long-only `--verbose` flag enables execution tracing
      for `ask`, `validate`, `map`, `rate`, `reduce`, `rank`, and `gate`; `-v`
      remains the version flag.
- [ ] Without `--verbose`, stdout, stderr, exit codes, request counts, retry
      behavior, ordering, and partial-output semantics remain byte-compatible
      with current behavior.
- [ ] With `--verbose`, result data remains exclusively on stdout and is
      byte-identical to the same successful invocation without verbosity.
- [ ] JEQ remains stateless: it never creates, discovers, reads, or updates a
      memory, history, trace, session, or run-state file. A later invocation
      sees prior evidence only when the caller explicitly supplies it as input.
- [ ] Trace events are ephemeral single-line JSON objects on stderr using the versioned
      schema identifier `jeq.trace.v1`; existing final `Error: ...` rendering
      remains on stderr and keeps its current exit mapping.
- [ ] Root and command help explain that verbose traces contain execution
      metadata, not model reasoning or raw payloads.

### Correlation and determinism

- [ ] `--trace-id` and `JEQ_TRACE_ID` accept a caller-provided opaque correlation
      value; flag precedence is explicit and matches established flag-over-env
      behavior.
- [ ] Trace IDs are validated before input, credential, or network access,
      limited to 128 characters, and restricted to a documented safe character
      set.
- [ ] When no trace ID is supplied, events remain useful within one process
      without requiring a random identifier; whole-pipeline correlation is
      documented as requiring an explicit inherited trace ID.
- [ ] Each process emits monotonically increasing `sequence` values, a stable
      `command`, and an `entry_point: "cli"`; event ordering follows input order
      and never depends on goroutine scheduling.
- [ ] Wall-clock timestamps and durations may be observational fields, but
      correctness and golden tests never depend on their exact values.

### Stable event vocabulary

- [ ] The initial schema uses a small documented vocabulary: `run.started`,
      `preflight.completed`, `operation.started`, `request.attempted`,
      `request.retrying`, `operation.completed`, `output.written`, `run.failed`,
      `events.suppressed`, and `run.completed`.
- [ ] Every event contains only applicable typed fields from an allowlist,
      including schema, sequence, trace ID when supplied, entry point, command,
      event, phase, operation index, bounded counts, question names/types,
      framing, pointer, model and model source, attempt budget, HTTP status
      class/code, stable JEQ error code, and outcome.
- [ ] Events do not use interpolated prose as the primary data contract.
- [ ] Existing retry diagnostics originate from typed retry information rather
      than parsing transport log strings; default non-verbose retry messages
      remain compatible.

### Command lifecycle coverage

- [ ] `ask` and `validate` trace one preflight/evaluation lifecycle and native
      versus composed request mode without exposing payloads.
- [ ] `map` and `rate` trace one operation per record, including one-based index,
      total when known, request attempts, output writes, and final
      seen/succeeded/emitted/failed counts.
- [ ] `reduce` traces collection framing and item count followed by one request
      lifecycle and one output.
- [ ] `rank` traces candidate count followed by one Choice request lifecycle and
      one output; candidate identifiers are not logged.
- [ ] `gate` traces offline policy evaluation and aggregate pass/ambiguous/reject
      counts without claiming a network request.
- [ ] Source conflicts, malformed framing, invalid pointers, evidence collisions,
      missing authentication, API rejection, retry exhaustion, timeout,
      interruption, malformed response, output failure, and partial-stream
      failure each emit a stable `run.failed` event with phase, operation index
      when applicable, and JEQ error code before the existing final error.
- [ ] A partial `map` or `rate` failure preserves all prior stdout exactly and
      names only the failed record index in the trace.

### Boundedness and privacy

- [ ] Detailed per-operation and per-attempt events have a fixed documented
      budget. After the budget, JEQ emits one `events.suppressed` event and still
      emits failures and the final summary.
- [ ] Event volume and memory use remain bounded at maximum supported record and
      retry counts.
- [ ] Traces never contain API keys, authorization/header values, config file
      contents, raw state or records, candidate IDs, instructions, criteria,
      prompts, raw request/response bodies, token text, raw provider error
      bodies, or URLs with user info, query strings, or fragments.
- [ ] Traces do not emit hashes of state, prompts, or low-entropy personal data.
- [ ] Endpoint visibility is limited to a sanitized scheme and host when useful.
- [ ] Tests seed secrets, personal data, prompt text, criteria, candidate IDs,
      response bodies, and credential-bearing URLs in every relevant layer and
      prove none appears in verbose stderr.

### Architecture

- [ ] A typed observer/trace-sink port with a no-op default crosses CLI,
      application pipeline, and HTTP adapter boundaries; commands do not format
      ad hoc trace strings.
- [ ] The CLI stderr adapter owns `jeq.trace.v1` serialization and allowlist
      enforcement.
- [ ] `processMapInput` supplies record lifecycle context shared by `map` and
      `rate`; request/retry instrumentation is reused by `ask`, `reduce`, and
      `rank`; `gate` emits local policy events without transport coupling.
- [ ] Existing dependency arrows remain intact: domain/application code does not
      import Cobra, OS logging, or the HTTP adapter.

### Verification and documentation

- [ ] Unit tests cover event serialization, field allowlisting, trace-ID
      validation/precedence, sequence ordering, event suppression, and no-op
      behavior.
- [ ] Built-binary fake-endpoint tests compare verbose-on/off stdout bytes and
      request counts for `ask`, two-record `map`/`rate`, `reduce`, `rank`, and
      offline `gate`.
- [ ] Built-binary tests cover retry-then-success, retry exhaustion, auth and
      validation failures before network, malformed response, timeout,
      interruption, output failure where practical, and partial-stream failure.
- [ ] A built shell-chain test proves an inherited trace ID correlates separate
      commands while every stdout stage remains valid JSON/NDJSON and no trace
      line enters the data pipe.
- [ ] README, architecture documentation, root help, and a self-contained
      executable `jeq examples debug-chain` recipe explain capture, correlation,
      privacy boundaries, event schema, and interpretation alongside result
      evidence.
- [ ] `nix develop -c make check` passes without live or paid requests.

## Delivery slices

1. **Trace contract:** typed events, safe serializer, no-op observer, trace-ID
   validation, budgets, and unit/privacy tests.
2. **CLI and transport:** global flags, stderr sink, run/preflight events, typed
   retry bridge, `ask`/`validate` lifecycle, and compatibility tests.
3. **Probabilistic operations:** shared `map`/`rate` record lifecycle plus
   `reduce`, `rank`, and offline `gate` summaries and failure phases.
4. **Chain verification:** built-binary failure matrix, cross-process trace
   correlation, docs, architecture map, and executable `debug-chain` recipe.

Each slice must be a forward commit with focused passing checks. The feature is
not complete until every command and privacy gate passes independent QA.

## Constraints

- Observability must not alter business decisions, probabilities, output order,
  request shape, request count, retries, or policy exits.
- Keep telemetry ephemeral on stderr; do not add local persistence, a state
  directory, daemon, remote collector, metrics backend, OpenTelemetry
  dependency, or automatic upload. Shell redirection is solely caller-owned.
- Keep existing lossless response envelopes as the single semantic evidence
  channel.
- Keep tests offline, deterministic, and independent of paid TypeSafe calls.

## Non-goals

- Exposing or reconstructing private model chain-of-thought.
- Logging raw payloads under a more alarming flag.
- Automatically deciding that a low-confidence judgment is wrong.
- Changing prompts, criteria, thresholds, ranking, or retry policy.
- Persistent memory, run history, telemetry storage, automatic trace discovery,
  dashboards, alerts, aggregate fleet metrics, or distributed tracing export.
- A second human output format for evaluation results.

## Follow-up boundary

If lifecycle traces and existing evidence are insufficient to debug generated
requests, plan a separate explicit offline request-inspection feature. It must
require deliberate invocation and apply its own state-disclosure warnings;
verbose mode must remain safe for routine capture.
