# ADR 0004: composable decision envelope v1

- Status: accepted
- Date: 2026-09-19

## Decision

`internal/domain/pipeline` implements one deterministic record transformation.
Given a strict JSON object, a JSON Pointer, a name, questions, and an injected
TypeSafe evaluator, it selects one state value, performs exactly one normal
`jeq ask` evaluation, and appends the complete typed response at
`_jeq.<name>`. The original record remains the envelope for the next Unix
operation:

```text
record -> select pointer -> one typed judgment -> record + _jeq.<name>
                                  |
                         pipe the record forward
```

### Envelope v1

- The input is a strict JSON object. Duplicate keys are rejected recursively.
- `_jeq` is absent or an object. It is created when absent; an existing
  `_jeq.<name>` is never overwritten and is a local input error.
- Names are ASCII identifiers: one leading letter or digit, followed by ASCII
  letters, digits, `_`, or `-`, at most 64 bytes.
- `pointer` is RFC 6901. The empty pointer selects the whole record. `~0` and
  `~1` are decoded, arrays require canonical indexes (`0` or a non-zero digit
  sequence), and `-` is not an index.
- The selected value must satisfy the existing TypeSafe state contract:
  string, object, or array. Missing members, malformed pointers, scalar
  traversal, null/number state values, and unsupported roots fail locally.
- The appended value is the complete decoded response: typed answers, model,
  usage, and unknown response fields. Input values are held as raw JSON so
  numeric lexemes, Unicode, control characters, and unknown members survive.

The engine has no branches, actions, templates, path lookup, environment
expansion, or model authority. Model answers are evidence only; neither answer
keys nor response fields become paths or commands. Callers must not pipe
secrets into logs: v1 intentionally carries the original record forward, while
future CLI projection belongs to standard tools and a separate task.

## Port and error boundary

The evaluator port is the existing `Evaluate(context.Context, contract.Request)`
shape used by the TypeSafe client. The domain package imports only the standard
library and domain contract/error packages. It does not know Cobra, files,
stdin, HTTP, or rendering. Evaluator authentication, interruption, timeout, and
other operational errors are returned unchanged. Local envelope/pointer errors
use `JEQ_INPUT_INVALID` before evaluator construction.

## Why this instead of workflows

A composability law keeps each semantic operation visible, independently
bounded, and easy to compose with Unix tools. It preserves the input needed by
the next operation and avoids hiding intermediate judgments behind a workflow
interpreter. A future workflow feature should compile to these visible
one-record primitives before it receives a separate abstraction; v1 therefore
explicitly defers branching, iteration, stage orchestration, policy exits, and
side effects.

JSON is used because it is the existing TypeSafe contract, supports raw-value
preservation through `json.RawMessage`, and composes with `jq`, NDJSON, and the
existing CLI without introducing a language or template engine.
