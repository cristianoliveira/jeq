---
id: TASK-0051
title: Rewrite README in Cristian's voice
status: doing
depends_on: []
priority: normal
tags: [docs, readability]
---

# Rewrite README in Cristian's voice

## Problem
README uses abstract terms such as “command surface” and “boundaries”, and the same polished tone appears across the page. Cristian wants the whole README to sound direct, practical, and human.

## Desired outcome
README explains what jeq does, how to install and discover it, and what users must consider in Cristian's documentation voice: function first, concrete actions, real constraints, and no corporate or generic LLM phrasing.

## Acceptance criteria
- [ ] Rewrite all root README prose, headings, and slogan where needed, not only the two rejected terms.
- [ ] Start from what jeq does and why piping typed JSON helps.
- [ ] Explain installation choices as direct actions.
- [ ] State that jeq explains itself through native help and recipes, so no jeq-specific skill is needed.
- [ ] State network, trust, privacy, cost, exit-code, and ownership constraints in concrete language.
- [ ] Preserve commands, URLs, technical facts, and lowercase `jeq` naming.
- [ ] Do not use em dashes, abstract consultant language, forced catchphrases, or inflated claims.
- [ ] Documentation checks pass.

## Non-goals
- Do not change CLI behavior or rewrite focused guides.

