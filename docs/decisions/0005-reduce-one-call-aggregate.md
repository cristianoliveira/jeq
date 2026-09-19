# ADR 0005: Reduce is one bounded aggregate request

- Status: accepted
- Date: 2026-09-19

## Decision

`gev reduce` accepts one bounded JSON collection and sends exactly one TypeSafe request. JSON input must be an array; NDJSON values are collected in input order into an array. The request state is that exact collection and the output is one `{items,_gev}` envelope.

Reduce is not an iterative fold or runtime DSL. It does not make one request per item, infer an accumulator, or execute user-provided code. This preserves deterministic request counts, bounded memory, and the same strict question-document contract used by `map`.

The model precedence for `map` and `reduce` is `--model`, `TYPESAFE_DEFAULT_MODEL`, user config, then `jev-latest`. User config is limited to `default_model` under `${XDG_CONFIG_HOME:-$HOME/.config}/gev/config.json`; an explicit `--config` path is strict and required to exist. `ask` and `validate` retain their existing resolution behavior for compatibility.
