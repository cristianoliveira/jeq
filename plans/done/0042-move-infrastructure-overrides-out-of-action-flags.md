---
id: TASK-0042
title: Move infrastructure overrides out of action flags
status: done
depends_on: []
priority: high
tags: [cli, configuration, composability]
---

# Move infrastructure overrides out of action flags

## Problem

JEQ repeats `--base-url` and `--config` across probabilistic action commands
even though endpoint and config-file selection are process infrastructure. This
clutters agent-facing help, complicates chains, and exposes testing seams as
question-level UX.

`--config` is also inconsistent: it appears on `map`, `reduce`, `rank`, and
`rate`, but not `ask`; `--base-url` appears on every network command. A shell
pipeline should inherit one infrastructure environment without repeating flags.

## Desired outcome

Infrastructure is configured once through the process environment:

```sh
export TYPESAFE_BASE_URL=http://127.0.0.1:8080
export JEQ_CONFIG="$PWD/jeq.config.json"

jeq map ... |
  jeq rate ... |
  jeq reduce ...
```

The action-command surface contains probabilistic intent and deliberate runtime
controls, not endpoint or config-path selection.

## Configuration contract

### Config path

1. Non-empty `JEQ_CONFIG` names an explicit config file.
2. Otherwise JEQ optionally reads
   `${XDG_CONFIG_HOME:-$HOME/.config}/jeq/config.json`.
3. Otherwise built-in defaults apply.

An explicit `JEQ_CONFIG` path must exist and contain a valid strict config
document. The config schema remains deliberately small:

```json
{"default_model":"jev-latest"}
```

### API root

1. Non-empty `TYPESAFE_BASE_URL` overrides the API root for the process.
2. Otherwise JEQ uses its built-in TypeSafe API root.

The API root does not need a config-file field in this task. Scripts, local fake
endpoints, staging, and tests use the environment override.

### Model

For composed requests, model resolution remains:

1. `--model`
2. `TYPESAFE_DEFAULT_MODEL`
3. selected user config `default_model`
4. built-in default

Native requests retain the model embedded in the request document. `ask` joins
the same composed-request config resolution used by `map`, `reduce`, `rank`, and
`rate`.

## Acceptance criteria

### Public CLI surface

- [x] Remove `--base-url` from `ask`, `map`, `reduce`, `rank`, `rate`, and
      `models`; passing it produces the normal unknown-flag usage failure.
- [x] Remove `--config` from `map`, `reduce`, `rank`, and `rate`; passing it
      produces the normal unknown-flag usage failure.
- [x] Root and command help no longer advertise either flag.
- [x] Keep `--model`, `--timeout`, and `--max-retries` unchanged; their product
      design is outside this task.

### Environment resolution

- [x] Add `JEQ_CONFIG` as the only explicit config-path override.
- [x] A non-empty `JEQ_CONFIG` takes precedence over XDG/HOME discovery for all
      composed network commands, including `ask`.
- [x] An explicit missing, unreadable, oversized, malformed, duplicate-key, or
      unsupported-field config fails before credential lookup, client creation,
      or network access.
- [x] When `JEQ_CONFIG` is absent or blank, optional XDG/HOME discovery retains
      current behavior; a missing default config is not an error.
- [x] `TYPESAFE_BASE_URL` remains the only API-root override for `ask`, `map`,
      `reduce`, `rank`, `rate`, and `models`.
- [x] A blank or absent `TYPESAFE_BASE_URL` uses the built-in root.
- [x] API-root validation and errors are consistent across every network
      command and occur before credential lookup or client creation where
      current source-order guarantees require it.

### Pipeline behavior

- [x] One exported `JEQ_CONFIG` and `TYPESAFE_BASE_URL` apply to every JEQ
      process in a shell pipeline without repeated arguments.
- [x] Native `ask --request` and native `map --request-pointer` continue using
      the request model and do not fail merely because a default-model config is
      selected.
- [x] Removing the flags does not change request shape, evaluation result,
      retry behavior, stdout/stderr separation, exit codes, or partial-output
      semantics.

### Architecture

- [x] Centralize config-path and API-root resolution rather than copying
      environment lookup across commands.
- [x] Keep environment access behind existing injected dependencies so tests do
      not mutate the real process environment.
- [x] Fake-endpoint tests use `TYPESAFE_BASE_URL` or dependency injection, not a
      public production flag.
- [x] The config remains free of credentials; `TYPESAFE_API_KEY` stays an
      environment-only secret.

### Verification and documentation

- [x] Unit tests cover precedence, blank values, explicit-path failures,
      optional default absence, and composed/native distinctions.
- [x] Built-binary fake-endpoint tests cover every network command through
      `TYPESAFE_BASE_URL`, prove removed flags fail before any request, and prove
      a multi-process chain inherits both environment settings.
- [x] Help-surface tests assert the flags are absent from every affected command.
- [x] README and architecture documentation describe the environment-only
      infrastructure contract and updated model precedence.
- [x] Existing examples stop passing `--base-url`; offline fake-endpoint tests
      remain deterministic and make no live or paid calls.
- [x] `nix develop -c make check` passes.

## Constraints

- Prefer one obvious configuration path over deprecated aliases; this
  pre-release CLI does not retain hidden compatibility flags.
- Validate configuration before credentials and network whenever the command's
  existing source-order contract permits it.
- Keep result streams lossless and diagnostics on stderr.
- Tests remain offline and deterministic.

## Non-goals

- Moving API keys into the config file.
- Adding named profiles or contexts.
- Adding `base_url`, timeout, or retry fields to the config schema.
- Removing the per-operation `--model` override.
- Redesigning timeout or retry configuration.
- Implementing TASK-0041 verbose observability.
