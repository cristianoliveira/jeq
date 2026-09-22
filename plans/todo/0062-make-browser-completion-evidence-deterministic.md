---
id: TASK-0062
title: Make browser completion evidence deterministic
status: doing
depends_on: [TASK-0061]
priority: high
tags: [examples, jev, playwright, determinism, paid-test]
---

# Make browser completion evidence deterministic

## Problem
A paid browser showcase reached the correct Casa Flora detail page but Jev selected BLOCKED because the policy state included URL, title, and actionable elements without bounded visible body evidence. Completion therefore depended on inference rather than observable proof.

## Desired outcome
Each Jev decision receives a bounded representation of the actual visible body text already captured by independent browser inspection. On the final detail page, the state explicitly contains every fact needed to choose DONE instead of guessing from title and URL.

## Acceptance criteria
- [ ] Pass visible body evidence from the runner into the policy plan on every cycle.
- [ ] Bound body evidence before serialization; do not expose an unbounded page or hidden DOM.
- [ ] Keep fixture verification independent from the Jev decision.
- [ ] Unit tests prove body evidence is present, bounded, and excluded from safe trace output.
- [ ] Offline fake-provider E2E still passes the complete six-decision flow.
- [ ] Fresh watcher passes at a clean committed HEAD.
- [ ] Run one user-authorized paid retry only after offline gates pass, using a headed Playwright CLI browser when the environment supports it.
- [ ] Paid headed retry reaches DONE and independently verifies URL, title, stay, destination, category, and cancellation.
- [ ] No example browser session or fixture server remains afterward.

## Evidence
- Failed paid trace: `output/playwright/playwright-browser-agent/paid-run-2.log`.
- The browser reached Casa Flora, then Jev selected BLOCKED with no final body facts in its request state.

