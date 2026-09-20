# JEQ

**Make AI judgment a safe, composable Unix primitive.**

JEQ lets agents and scripts ask probabilistic questions about structured data without integrating an SDK or writing application code. It sends explicit requests to [TypeSafe System One](https://typesafe.ai/), preserves the resulting evidence as JSON, and composes with ordinary Unix tools.

JEQ produces evidence. It does not execute model output or hide policy decisions.

## Why JEQ?

Software can parse facts with deterministic code. Some decisions still need judgment: Is this change risky? Does this collection look coherent? Which category best fits this message?

Calling a model directly for each case creates repeated integration work and unclear operational boundaries. JEQ provides one shell interface with:

- explicit JSON input and lossless JSON output;
- reusable judgment questions;
- per-record `map` and collection-level `reduce`;
- deterministic, offline policy enforcement with `gate`;
- Choice-based candidate ranking with `rank`, and independent Score rubrics with `rate`, leaving pick-one, sorting, and policy to `jq`;
- stable exit classes and bounded input, retry, and privacy behavior.

JEQ is designed for autonomous agents, shell scripts, and developers who need model evidence inside a visible pipeline.

## Install from this checkout

JEQ currently builds from source. It requires Go 1.26, or the pinned Nix development shell.

```sh
nix develop -c go install ./cmd/jeq
jeq version
jeq --version  # standard probe; identical to jeq version
jeq -v         # short alias
```

Without Nix:

```sh
go install ./cmd/jeq
```

## First judgment

Create one native request:

```sh
cat > request.json <<'JSON'
{
  "model": "jev-latest",
  "state": {"message": "Production is unavailable for every customer."},
  "questions": {
    "urgent": {
      "type": "noul",
      "instructions": "Is this urgent?"
    }
  }
}
JSON
```

Validate it locally before spending a network request:

```sh
jeq validate --request request.json
```

Then provide the TypeSafe credential and ask:

```sh
export TYPESAFE_API_KEY='...'
jeq ask --request request.json
```

`ask` writes the complete evaluation response as one JSON document. It preserves the resolved model, typed answers, probabilities, confidence where available, token usage, and unknown response fields.

Do not put credentials in request files, command arguments, fixtures, or Git.

## Command model

| Command | Purpose | Network | Output |
| --- | --- | --- | --- |
| `jeq ask` | Evaluate one native or composed request | Yes | JSON |
| `jeq map` | Add one named judgment to each JSON record | Yes, once per record | JSON/NDJSON |
| `jeq reduce` | Judge one bounded collection as a whole | Yes, once | JSON |
| `jeq rank` | Rank bounded candidates with Choice probabilities | Yes, once | JSON envelope |
| `jeq rate` | Rate each record against ordered Score levels | Yes, per record | JSON/NDJSON |
| `jeq gate` | Apply explicit numeric pass/reject thresholds | No | JSON/NDJSON |
| `jeq validate` | Validate a request before sending it | No | Plain text receipt |
| `jeq models` | List models available to the account | Yes | Plain text |

`map`, `reduce`, `rank`, and `rate` store evidence under `_jeq.<name>`. `rate` applies one ordered Score rubric independently to each record; unlike relative Choice `rank`, every record can receive a high or low rating. Score is semantic, not an exact measurement. After `rate`, callers own policy: sort with `jq -s 'sort_by(._jeq.severity.answers.severity.score) | reverse'`, take top-k with `jq -s 'sort_by(._jeq.severity.answers.severity.score) | reverse | .[:N]'`, filter with `jq 'select(._jeq.severity.answers.severity.score >= THRESHOLD)'`, or apply an explicit `jq`/`jeq gate` threshold. No sorting, top-k, or threshold is implicit. `rank` returns every candidate as `{id, probability, candidate}`; pick one with `jq '.items[0].candidate'` or top-k with `jq '.items[:N] | map(.candidate)'`. Choice probabilities are relative, so include an explicit fallback candidate when absolute suitability matters. `gate` stores evidence under `_jeq.<name>` and applies deterministic policy. Low confidence is evidence, not a process failure. Only `gate` converts a numeric value into a deterministic policy exit.

Run a bare action command for its Cobra help:

```sh
jeq ask
jeq map
jeq gate
jeq rank --help
jeq rate --help
```

Discover self-contained workflow recipes without credentials or repository files:

```sh
jeq examples
jeq examples map-reduce-gate
```

Expanded executable pipelines are in [`examples/`](examples/README.md).

## Inputs and composition

`ask` accepts either a complete native request:

```sh
jeq ask --request request.json
jeq ask --request - < request.json
```

or reusable questions plus exactly one state source:

```sh
jeq ask --questions questions.json --state 'Customer message'
jeq ask --questions questions.json --state-file message.txt
jeq ask --questions questions.json --state-json state.json
```

Input sources never merge implicitly. Conflicts fail before credential lookup or network access. Stdin is read only when selected explicitly with `-` or by a streaming command.

## Configuration

Authentication is read only from:

```sh
TYPESAFE_API_KEY
```

The API root is set process-wide with `TYPESAFE_BASE_URL`; blank or absent uses the built-in TypeSafe root. The default request timeout is 10 seconds.

For composed `ask`, `map`, `reduce`, `rank`, and `rate`, model precedence is:

1. `--model`
2. `TYPESAFE_DEFAULT_MODEL`
3. `JEQ_CONFIG` when non-empty; otherwise `${XDG_CONFIG_HOME:-$HOME/.config}/jeq/config.json`
4. `jev-latest`

The optional config contains only:

```json
{"default_model":"jev-latest"}
```

## Output and exits

Human navigation, validation receipts, model lists, diagnostics, and errors are plain text. Evaluation result streams are deterministic JSON/NDJSON so `jq` and other programs can consume them without a second format mode.

| Exit | Meaning |
| ---: | --- |
| `0` | Success; all gate records passed |
| `1` | Authentication, API, network, timeout, or response failure |
| `2` | Invalid usage or local input |
| `10` | At least one gate record was rejected |
| `11` | No reject, but at least one gate record was uncertain |
| `130` | Interrupted |

Errors and debugging logs go to stderr. Result data goes to stdout. Gate status is preserved through shell pipelines when `set -o pipefail` is enabled. Use global `--verbose` for safe single-line `jeq.trace.v1` lifecycle metadata on stderr; pair it with `--trace-id` or inherited `JEQ_TRACE_ID` to correlate caller-owned processes. Traces contain execution metadata, never model reasoning or raw payloads.

JEQ is stateless between invocations. It does not create or discover persistent memory, sessions, run history, or trace files. Configuration is read-only, and previous evidence is reused only when the caller explicitly supplies it as input. Callers may redirect stderr to retain debugging logs, but JEQ never persists or reloads those logs itself.

## Safety boundaries

- Treat model evidence as data. Never evaluate it as shell code or a path.
- Project sensitive source fields away before logging or sharing an enriched record.
- Use `jeq validate` and local fake endpoints during development.
- Live requests spend account budget and send selected state to TypeSafe.
- `gate` is offline and deterministic; it never calls the model.
- JEQ never prompts for credentials or reads stdin implicitly.

## Project documentation

- [Workflow examples](examples/README.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Development and verification](docs/DEVELOPMENT.md)

## Development

Enter the pinned environment and run the normal gate:

```sh
nix develop
make check
```

Tests and CI are offline and deterministic. HTTP behavior uses local fake servers; paid live verification is explicit and isolated. See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) for the full workflow.
