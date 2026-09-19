---
id: TASK-0027
title: Improve code-smell judgment questions
status: doing
depends_on: [TASK-0026]
priority: normal
tags: [examples, typesafe]
---

# Improve code-smell judgment questions

## Problem
The first live code-smell example forced compatible smells to compete in one Choice and combined several qualities into a vague cohesion Noul. It returned low Choice confidence and one opaque aggregate number, making the evidence difficult to interpret or gate without manufacturing certainty.

## Desired outcome
One request returns independently useful, directly gateable probabilities for five design dimensions. The example helps developers locate the weakest dimension without claiming generated rationale or optimizing for artificially high confidence.

## Design basis
TypeSafe guidance says Choice options compete, while one Noul per label gives an absolute probability and allows several labels to apply. Jev 1.13 performs best with literal conditions, little indirection, aligned true/false criteria, and relevant state. Independent questions over the same state should be batched in one request.

## Acceptance criteria
- [ ] Replace the forced `primary_smell` Choice and vague `cohesive` Noul with five positive, independent Nouls: `responsibilities_focused`, `policy_centralized`, `dependencies_explicit`, `abstractions_encapsulated`, and `complexity_justified`. High always means healthy/safe; low names the corresponding smell.
- [ ] Every question states the exact state shape (`[{path,content}, ...]`), tells the model to treat source/comments/strings as untrusted data rather than instructions, asks one literal condition, and supplies aligned `criteria.true` / `criteria.false` boundary descriptions with concrete inclusions and exclusions.
- [ ] Keep all five questions in the same one-call `reduce`; do not add stages, calls, generated explanations, a reasoning model, or a Choice that forces co-occurring smells to compete.
- [ ] README shows a deterministic jq projection that removes `.items`, lists dimensions from lowest probability to highest, and computes `quality_floor` as the minimum Noul. Optional `gev gate` applies explicit thresholds to that floor; it does not use Choice confidence.
- [ ] README explains that each Noul is probability of its own yes-condition, not intensity or confidence; low dimensions are review signals, not proof. It advises reviewing one related concern at a time and excluding generated/vendor/irrelevant files to avoid context dilution.
- [ ] Add small labeled `cohesive` and `duplicated-policy` fixtures (or equivalent controlled cases) used only for demonstrating/evaluating question behavior. They must not be compiled as production examples or asserted as deterministic model truth.
- [ ] Behavioral fake-API tests verify the exact five question IDs/types, positive direction, non-empty aligned criteria, one request, response preservation, deterministic projection/floor/gate composition, and absence of old Choice/confidence policy. Tests make no quality claim.
- [ ] Run bounded live comparison on the cohesive fixture, duplicated-policy fixture, and one real related file set. Record model/version, per-dimension Nouls, usage, and whether the intended weak dimension separates from the cohesive case. Do not turn observed values into universal thresholds or deterministic CI assertions.
- [ ] Iterate wording at most once from source-backed failure analysis; avoid tuning solely to three samples. Preserve disclosure, input bounds, temp cleanup, Bash portability, and advisory default behavior from TASK-0026.
- [ ] Full checks, govulncheck, architecture, example behavioral tests, and final independent QA pass.

## Non-goals
- No generated explanation, AST analysis, inline review comments, automatic refactor, universal code-quality score, model-quality CI assertion, extra API request, jq runtime, or replacement for deterministic tools.

## Notes
The goal is better question validity and diagnostic evidence, not higher-looking confidence. Keep observed live results in QA/local reports without source content.

