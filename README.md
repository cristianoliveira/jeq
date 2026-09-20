<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/jeq-logo.svg">
  <img src="docs/assets/jeq-logo-mono.svg" alt="JEQ logo" width="160" height="160">
</picture>

# JEQ

**Intelligence you can pipe**

*Judgment as a Unix primitive for scripts, agents, and systems.*

Some questions are easy to express with deterministic code. Others are not: Is this change risky? Does this collection look coherent? Which issue should we look at first?

The awkward part is that exploring one of these questions usually means writing an integration before we even know if the idea is useful.

JEQ makes probabilistic judgment a Unix primitive. It lets you try an idea against real JSON, compose it with `jq` and the shell, and keep the resulting evidence separate from the decision you make with it.

Use JEQ when you want to:

1. **Experiment quickly.** Ask a typed question without adding an SDK, service, or application code.
2. **Compose freely.** Mix judgment with pipes, files, `jq`, shell conditions, and the tools you already have.
3. **Automate without hiding the risky part.** Preserve probabilities as evidence, then keep thresholds, fallbacks, and side effects explicit.

JEQ sends requests to [TypeSafe System One](https://typesafe.ai/). It does not execute model output, invent policy, or turn a shell experiment into a hidden agent runtime.

## A judgment layer for Unix

A useful way to think about JEQ is `jq` for questions that need judgment. Not as a replacement for `jq`, because they do different jobs.

- `jq` selects fields, reshapes records, joins data, sorts results, and applies exact rules.
- `jeq` classifies, rates, ranks, or judges those records.
- The shell decides how evidence moves between steps.
- Your code still decides what happens next.

For example, this pipeline selects issue data, rates every issue against the same urgency rubric, then uses `jq` to choose the five highest scores:

```sh
jq -c '.issues[] | {id, title, description}' issues.json |
  jeq rate \
    --as urgency \
    --input ndjson \
    --state-pointer /description \
    --instruction 'How urgent is this issue?' \
    --level Low \
    --level Medium \
    --level High |
  jq -s 'sort_by(._jeq.urgency.answers.urgency.score) | reverse | .[:5]'
```

That division matters. `jq` handles the exact work. JEQ supplies judgment evidence. The final top-five rule belongs to the caller, where it stays visible and easy to change.

## Start with an idea, not an integration

Most AI experiments start too big. You create a client, decide where state lives, add retries, design an output type, and only then discover whether the question was useful.

With JEQ, the first version can be one command. Feed it a fixture, inspect the JSON, change the question, and run it again. If the idea survives contact with real data, the same contract can move into a script, CI job, or agent workflow.

This makes a few useful loops cheap:

- sample a large input with `head` before spending requests on the whole set;
- compare question or rubric changes against the same fixtures;
- project sensitive or irrelevant fields away before sending state;
- save evidence as ordinary JSON and inspect it with normal development tools;
- switch the selected model without rewriting the pipeline;
- validate a request locally before making a network call.

JEQ stays stateless while you do this. Files, history, and replay belong to the caller, so an experiment does not quietly become another system to operate.

## Composition is the workflow

JEQ has a small set of verbs with different semantics and cost shapes:

| Primitive | Use it when | Request shape |
| --- | --- | --- |
| `map` | Every record needs the same named judgment | One request per record |
| `rate` | Every record needs an independent score against an ordered rubric | One request per record |
| `rank` | Candidates need a relative comparison | One request for the bounded set |
| `reduce` | A collection needs one aggregate judgment | One request for the bounded collection |
| `gate` | Numeric evidence needs an explicit pass, uncertain, or reject policy | Offline |
| `validate` | A request contract should be checked before any spend | Offline |

Because evaluation results are JSON, these primitives can stay small. The shell already knows how to do the rest:

- enrich records in stages with `map`;
- join a classification with a local catalog using `jq`;
- use `rate` and let `jq` filter, sort, or take the top results;
- use `rank` when the question is relative rather than absolute;
- summarize local findings with `reduce`;
- branch the same input with `tee` to compare different questions;
- finish with `gate` when a numeric policy must control the exit status;
- keep failures visible with `set -o pipefail`;
- correlate several JEQ processes with `--trace-id` when a pipeline needs debugging.

The thing is, composition only works when commands do not take ownership away from each other. JEQ therefore keeps result data on stdout, diagnostics on stderr, and policy in explicit commands. It does not sort a rating, pick a ranked candidate, or treat low confidence as a hidden failure.

## Useful boundaries

JEQ deliberately does less than an agent framework:

- it produces typed evidence, not actions;
- it does not evaluate output as shell code, paths, or commands;
- it does not keep memory, sessions, or run history;
- it does not persist traces unless the caller redirects them;
- it does not hide network use behind an offline command;
- it does not decide that the highest score is automatically good enough.

Those constraints are part of the product. They make it possible to start with an experiment and understand what will still be true when it becomes automation.

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
