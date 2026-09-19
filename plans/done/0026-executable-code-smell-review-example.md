---
id: TASK-0026
title: Executable code-smell review example
status: done
depends_on: [TASK-0025]
priority: normal
tags: [examples]
---

# Executable code-smell review example

## Problem
Developers can run deterministic linters, but semantic smells such as mixed responsibilities, duplicated policy, hidden ambient behavior, and leaky abstractions still require judgment. gev now has map/reduce/gate primitives, but no safe copyable coding use case showing how to collect source files, ask one bounded aggregate question, and treat the result as advisory evidence.

## Desired outcome
A developer can pass an explicit bounded set of source files to one copyable command and receive typed, probabilistic evidence about the most material design smell plus whether the code is cohesive enough to leave unchanged. The example remains advisory and composes with an optional offline gate.

## Acceptance criteria
- [ ] `examples/code-smell-review/review.sh <file>...` converts only explicitly named readable regular files into ordered NDJSON `{path,content}` records and sends them through one `gev reduce --as code_smells --questions-json ...` invocation. It uses the configured model and makes exactly one API request.
- [ ] The embedded strict question document asks one Choice for `primary_smell` with bounded criteria `none`, `mixed_responsibilities`, `duplicated_policy`, `hidden_ambient_state`, `leaky_abstraction`, and `unnecessary_complexity`, plus one Noul named `cohesive` whose high value consistently means safe to pass through an optional gate.
- [ ] Input is bounded before network: require 1..20 arguments, reject missing/unreadable/non-regular files, reject files above 256 KiB, preserve argument order, handle spaces safely, and never discover or read repository files implicitly.
- [ ] The script is non-interactive, resolves `GEV_BIN` and `JQ_BIN` with useful defaults, preserves gev stdout/stderr/status, and emits no progress or source content outside the JSON envelope.
- [ ] README explains what probabilistic review adds beyond compiler/lint/static analysis, gives copyable explicit-file and `git diff` examples, shows safe projection that removes `.items` before logs, documents one request/cost and source-code disclosure, and warns against sending secrets or treating the judgment as proof.
- [ ] README shows optional `gev gate --value-pointer /_gev/code_smells/answers/cohesive/noul` with explicit pass/reject thresholds; the base review remains advisory and successful evaluation exits 0 regardless of answer.
- [ ] Behavioral tests execute the example against a compiled gev and deterministic fake API, assert exact one request, ordered path/content state, resolved default model, typed response/usage preservation, safe paths with spaces, and zero API calls for no args/missing/oversized inputs.
- [ ] A second behavioral test runs the documented inline-question form without a question file. No grep/regex documentation tests are added.
- [ ] Existing examples index links the use case. Full checks, shell formatting/lint where available, govulncheck, and an opt-in live run pass; live evidence records only answers/model/usage, not source content.

## Non-goals
- No recursive repository scan, git parser, AST/static analyzer, automated refactor, inline comments, CI blocking by default, jq runtime, or replacement for deterministic tools.

## Notes
Use the example to dogfood `reduce` and `--questions-json`. Keep collection and policy separate: reduce supplies evidence; callers explicitly choose whether to gate it.

