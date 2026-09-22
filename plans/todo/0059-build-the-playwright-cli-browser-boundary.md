---
id: TASK-0059
title: Build the Playwright CLI browser boundary
status: doing
depends_on: []
priority: high
tags: [examples, playwright, browser, security, testing]
---

# Build the Playwright CLI browser boundary

## Problem
A jeq browser-agent example needs a deterministic way to observe actionable elements and execute only observed actions through playwright-cli. Without an isolated browser boundary, paid semantic decisions would be mixed with brittle parsing, unsafe selector generation, and untestable browser mutations.

## Desired outcome
`examples/playwright-browser-agent` contains a small Python standard-library boundary that opens a named Playwright CLI session, captures an accessibility snapshot, turns supported observed refs into typed actions, executes one allowlisted action, and always closes the session. A local fixture and offline tests prove parsing, execution, cleanup, and independent outcome inspection before Jev is connected.

## Acceptance criteria
- [ ] Invoke the installed `playwright-cli` executable; do not import Playwright or copy jev-ultrafast's CDP/browser harness.
- [ ] Open only the caller-provided URL and use a unique named session.
- [ ] Parse actionable refs from actual Playwright CLI snapshot output for buttons, links, editable fields, checkboxes, and selects with observed options.
- [ ] Produce a finite typed action space. No model or caller string becomes a selector, JavaScript expression, shell command, or Playwright command name.
- [ ] Execute only allowlisted operations against a ref from the latest snapshot: click, fill, select, check/uncheck, bounded scroll/wait, or stop.
- [ ] Caller-supplied fill values are bounded data and never evaluated as code.
- [ ] Never retry a browser mutation automatically. A failed mutation returns evidence and stops.
- [ ] Enforce a fixed action budget and close the named browser session on success, failure, interruption, and timeout.
- [ ] Provide an independent page inspection function for final URL/title/body checks; a future model DONE decision is not success.
- [ ] Include a loopback-only fixture derived or recreated with clear attribution if copied from the MIT target project.
- [ ] Unit tests cover snapshot parsing, action allowlisting, stale/unknown refs, value boundaries, subprocess failure, and cleanup without paid calls.
- [ ] One real Playwright CLI smoke run against the local fixture proves observation, mutation, outcome inspection, and close.
- [ ] Watcher and independent QA pass.

## Constraints
- Preserve the source project's safety idea, not its Browser Harness implementation or performance claims.
- Do not add browser dependencies to the jeq production binary.
- Keep generated browser artifacts under `output/playwright/playwright-browser-agent/`.

## Evidence
- Source reference: `.local/jev-ultrafast` is MIT licensed; its core loop is page observation -> finite operation/target choice -> guarded execution -> independent verification.
- Offline tests must not call Jev or any text model.

