---
id: TASK-0048
title: Teach Jev's decision-model role in CLI help and examples
status: done
depends_on: []
priority: high
tags: [cli, documentation, examples, typesafe, jev]
---

# Teach Jev's decision-model role in CLI help and examples

## Problem
JEQ help names TypeSafe System One but does not explain that Jev is a fast semantic decision model, not a chat or generative LLM. Users may ask it for prose, code, or reasoning instead of defining bounded typed judgments that code can combine.

## Desired outcome
Every user who discovers JEQ through `--help` or `jeq examples` understands the correct mental model: Jev is “a classifier on steroids”—a System One semantic decision model that maps natural-language state into caller-defined typed answers and probabilities. It is not a chat/completion model, prose or code generator, or source of reasoning explanations.

## Context
Current TypeSafe documentation states that Jev understands natural-language input but returns typed decisions and probabilities rather than generated text. It does not write replies, produce code, or generate reasoning explanations. The caller defines answer spaces with Noul, Choice, and Score; code owns deterministic checks, workflow, thresholds, actions, and escalation.

Official sources consulted on 2026-09-20:
- https://docs.typesafe.ai/concepts/system-one.md
- https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md
- https://docs.typesafe.ai/primitives.md

The direct “classifier on steroids” phrase is a useful mental model, but always pair it with the precise System One definition so users do not mistake Jev for only a conventional label classifier.

## Acceptance criteria
- [ ] `jeq --help` prominently defines Jev's role before trace details: typed bounded decisions/probabilities from natural-language state, not generated prose/code or hidden reasoning.
- [ ] Help makes ownership explicit: caller defines state, answer space, policy, and actions; Jev supplies semantic judgment evidence.
- [ ] Network judgment command help (`ask`, `map`, `rate`, `rank`, `reduce`) reinforces the model where relevant without repeating a dense paragraph on every screen.
- [ ] `jeq examples` parent help and every recipe help contain a concise model-role note that connects Jev, the selected primitive, and caller-owned `jq`/shell policy.
- [ ] Add or update one focused guide that explains the “classifier on steroids” mental model, Noul/Choice/Score, good and bad task shapes, uncertainty, and escalation to a person or reasoning/generative model.
- [ ] README and examples index link the mental-model guide without materially expanding the landing page.
- [ ] Existing examples remain bounded typed judgments; wording does not imply Jev generates commands, prose, code, summaries, or chain-of-thought.
- [ ] Behavior tests prove root, relevant command, examples parent, and recipe help expose the model-role contract through native Cobra help.
- [ ] Tests and normal gate pass offline; no live TypeSafe request is required.

## Constraints and non-goals
- Do not claim Jev is a general-purpose LLM or a replacement for one.
- Do not claim an individual probability is guaranteed correct or universally calibrated for the caller's domain.
- Do not expose or request chain-of-thought.
- Keep help concise and scannable; deeper explanation belongs in the guide.
- Do not change request/response semantics or command behavior.

