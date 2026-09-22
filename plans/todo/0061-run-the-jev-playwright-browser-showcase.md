---
id: TASK-0061
title: Run the Jev Playwright browser showcase
status: doing
depends_on: [TASK-0059, TASK-0060]
priority: high
tags: [examples, jev, playwright, browser-agent, paid-test, documentation]
---

# Run the Jev Playwright browser showcase

## Problem
After the Playwright boundary and Jev decision policy exist independently, the examples list still needs one bounded end-to-end demonstration with documentation, a deterministic local fixture, independent verification, and explicitly approved paid evidence.

## Desired outcome
One documented command starts the loopback fixture and runs the installed `jeq` and `playwright-cli` binaries through the observe, decide, execute loop. The default goal completes several operation types, opens the correct detail view, verifies the actual final page independently, saves one useful artifact, and reports safe performance/cost evidence from an approved paid run.

## Acceptance criteria
- [ ] Wire TASK-0059 and TASK-0060 without duplicating their browser or decision logic.
- [ ] Each cycle observes, makes exactly one `jeq ask` decision request, executes at most one validated action, and observes again.
- [ ] Enforce fixed action/request/time bounds and close the browser session and fixture server on every exit path.
- [ ] Default goal requires fill, select, checkbox, submit/click, and final detail navigation without a fixture-specific action script.
- [ ] Fixture-specific code may start the server and independently verify destination, category, cancellation state, and selected item identity; model DONE alone is not success.
- [ ] Print compact step records with operation, observed target label, returned model/version, latency, and final PASS/FAIL. Do not print credentials, raw private state, or full provider bodies.
- [ ] Link the example from `examples/README.md` and explain how it reproduces and differs from jev-ultrafast.
- [ ] Credit the MIT source project and do not repeat its latency or reliability claims as jeq evidence.
- [ ] Offline behavior tests use a fake jeq executable and the local fixture; they assert actual final outcomes rather than prose.
- [ ] One explicitly approved paid run uses the configured TypeSafe provider against only the loopback fixture and records decision count, tokens, elapsed time, model versions, and verified outcome.
- [ ] Use Playwright CLI to save one screenshot or trace artifact of the verified final page, then confirm no example session remains.
- [ ] Watcher and independent QA pass.

## Constraints
- Do not claim generic autonomous browsing reliability from one local fixture.
- No separate generative text model. Fill values come from a finite caller allowlist.
- Do not expose the fixture server or browser control beyond loopback.
- No paid calls until offline tests and watcher are green.

## Implementation sequence
1. Add fake-jeq end-to-end fixture tests.
2. Add the bounded runner, verifier, docs, and examples-list entry.
3. Run offline and real Playwright local gates.
4. Run the approved paid demonstration and preserve safe evidence.
5. Request independent QA and close the task.
