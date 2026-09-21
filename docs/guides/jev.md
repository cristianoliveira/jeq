# Jev: semantic decisions for code

Jev is a **classifier on steroids**: TypeSafe System One maps natural-language
state to caller-defined typed decisions and probabilities. It is not a chat,
completion, prose, code, or reasoning-explanation model. Define the state and
answer space; your code owns deterministic policy, actions, and escalation.
Never ask for or expose hidden chain-of-thought.

## Three answer shapes

- **Noul**: probability of a bounded true/false condition. Ask “Is this ready?”
  rather than “Explain this release.”
- **Choice**: select among explicit alternatives, including a fallback. Choice
  probabilities are relative evidence, not universal confidence.
- **Score**: rate against ordered levels with named meanings. A score is a
  semantic rubric result, not an exact measurement.

Good: “Is this change safe to ship?” with `true`/`false` criteria and a local
gate. Bad: “Write a migration plan,” “summarize this,” or “show your reasoning.”
Those are prose or reasoning tasks; use a person or a reasoning/generative model
when that output is actually required, then validate it with deterministic code.

## Uncertainty and ownership

Probabilities are evidence for the question and domain you defined. They do not
guarantee correctness or imply universal calibration. Keep state bounded and
redacted, inspect the returned evidence, and choose explicit thresholds. Use a
person for high-impact ambiguity or judgment outside the answer space. Use a
reasoning/generative model for explanations or drafts, not as a hidden policy
engine. `jq`, the shell, and your application own projection, policy, retries,
and final actions.
