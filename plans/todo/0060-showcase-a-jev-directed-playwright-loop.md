---
id: TASK-0060
title: Showcase a Jev-directed Playwright loop
status: todo
depends_on: [TASK-0059]
priority: high
tags: [examples, jev, playwright, browser-agent, composition, paid-test]
---

# Showcase a Jev-directed Playwright loop

## Problem
The examples list does not show how jeq can reproduce the core jev-ultrafast pattern: observe a page, build a finite action space, let Jev choose one operation and compatible target in one request, execute through Playwright CLI, and verify the final outcome independently.

## Desired outcome
A documented runnable example uses the TASK-0059 browser boundary plus the installed `jeq` binary. Each cycle snapshots the current page, builds operation-specific Choice criteria, sends one native System One request through `jeq ask`, validates the chosen operation and matching target against the observed allowlist, executes it through Playwright CLI, and observes again. The bundled demo completes and independently verifies a local search/filter/open task.

## Acceptance criteria

### Jev policy
- [ ] Send operation and operation-specific target heads together in one `jeq ask` request per decision cycle.
- [ ] Target questions state their assumed operation because TypeSafe questions are independent.
- [ ] Consume only the target head matching the chosen operation; ignore unused speculative heads.
- [ ] Offer only operations available from the current snapshot plus bounded WAIT, scroll, DONE, and BLOCKED controls.
- [ ] Validate every Choice answer through jeq and again map the selected ID to the current in-memory allowlist before execution.
- [ ] Use caller-provided allowed text values as finite Choice criteria for fill actions. Jev selects data; it never generates executable text or selectors.
- [ ] Include current URL, title, visible snapshot text, element states, goal, and recent actions as bounded state.
- [ ] Stop on unknown output, command failure, action-budget exhaustion, BLOCKED, or failed final verification.

### Showcase
- [ ] Provide one command that starts the loopback fixture and runs the complete example with the real installed `jeq` and `playwright-cli` binaries.
- [ ] Default goal requires multiple operation types and ends on a detail view whose destination, category, cancellation setting, and item identity are verified independently.
- [ ] Print a compact step trace with operation, observed target, model/version, latency, and final PASS/FAIL without credentials or raw private state.
- [ ] Link the example from `examples/README.md` and explain how it reproduces and differs from jev-ultrafast.
- [ ] Credit the MIT source project and do not repeat its 7.1-second or reliability claims as jeq evidence.
- [ ] Offline behavior tests use a fake `jeq` decision subprocess and the local fixture; tests assert outcomes rather than prose.
- [ ] An explicitly approved paid run uses the configured TypeSafe provider against only the local fixture and records request count, token usage, elapsed time, and verified outcome.
- [ ] Save one useful Playwright screenshot or trace artifact for the verified demo, then close the browser session.
- [ ] Watcher and independent QA pass.

## Constraints
- No generic autonomous browsing claim. This is one bounded local demonstration.
- No separate generative text-model credential. The caller supplies a finite allowlist of fill values.
- No site-specific action script. Fixture-specific code may only start the server and verify the final outcome.
- Do not expose browser sessions or the fixture server beyond loopback.

## Implementation sequence
1. Add offline Jev-request construction and decision-validation tests over TASK-0059's typed action space.
2. Add the bounded observe/decide/execute loop and local demo runner.
3. Run offline tests and real local Playwright smoke checks.
4. Run one paid Jev demonstration, preserve safe evidence, then request independent QA.

