---
id: TASK-0060
title: Build a finite Jev browser policy
status: doing
depends_on: [TASK-0059]
priority: high
tags: [examples, jev, playwright, browser-agent, composition, testing]
---

# Build a finite Jev browser policy

## Problem
The approved Playwright boundary can observe and execute safe actions, but it has no semantic policy. Connecting the browser loop directly to live Jev before request construction and decision validation are independently tested would mix paid calls with unsafe target handling and make failures hard to diagnose.

## Desired outcome
A small policy module converts the latest TASK-0059 element snapshot into one bounded native System One request and converts a validated `jeq ask` response back into exactly one typed current action. The policy is fully testable with fixture responses and contains no browser subprocess or paid-call orchestration.

## Acceptance criteria
- [ ] Build one request containing an operation Choice plus operation-specific target Choice heads for the currently available click, fill, select, and check/uncheck actions.
- [ ] Target instructions explicitly name their assumed operation because TypeSafe questions are independent.
- [ ] Include only supported current elements, states, options, the bounded goal, and bounded recent action history.
- [ ] Offer bounded WAIT, scroll, DONE, and BLOCKED controls only where the policy can execute them.
- [ ] Represent refs and actions with opaque code-owned candidate IDs; criteria may describe refs, roles, names, values, and states, but model output never becomes a selector or command.
- [ ] Caller-provided allowed fill values form a finite Choice head. No model-generated text is accepted.
- [ ] Consume only the target/value heads matching the selected operation. Invalid unused speculative heads cannot execute.
- [ ] Reject missing answers, unknown operations, unknown/stale targets, incompatible target roles, unknown fill values, malformed response envelopes, and action-budget exhaustion before browser mutation.
- [ ] Preserve model/version, usage, and decision latency as safe metadata for the future step trace.
- [ ] Offline tests cover request shape, head independence, matching-head consumption, unused-head isolation, current allowlist mapping, state/history bounds, and every rejection path.
- [ ] Tests use fixture jeq responses and make no paid calls.
- [ ] Watcher and independent QA pass.

## Constraints
- Do not invoke Playwright CLI, start a server, or make a live provider call in this task.
- Use the installed `jeq ask --request -` contract at the adapter edge, but inject the subprocess runner in tests.
- Keep this example-local. Do not add browser policy to the jeq production binary.
- No site-specific operation sequence or fixture-specific target names in policy code.

## Implementation sequence
1. Add failing request-builder and decision-consumer tests.
2. Implement the bounded policy and injectable jeq subprocess adapter.
3. Run offline tests, watcher, and independent QA.
