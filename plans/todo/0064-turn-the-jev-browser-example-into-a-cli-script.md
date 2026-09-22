---
id: TASK-0064
title: Turn the Jev browser example into a CLI script
status: todo
depends_on: [TASK-0063]
priority: high
tags: [examples, jeq-cli, playwright-cli, shell, jev, paid-test]
---

# Turn the Jev browser example into a CLI script

## Problem
The current example has evolved into an SDK-shaped Python framework that supplies part of the route and hides the core jeq CLI interaction behind agent, policy, and runner abstractions. This obscures the intended proof: given a goal and the currently observed page, Jev predicts the next safe click.

## Desired outcome
A reader can open one executable shell script and see the complete Unix pipeline: observe with `playwright-cli`, build one finite Choice request with `jq`, call `jeq ask`, map the chosen candidate back to the latest observed ref, click it, and repeat. Starting from the Hacker News homepage, Jev chooses the route to yesterday's page, the most-discussed story, and completion from the goal and current observation.

## Approach
Replace the SDK-shaped Python layers with one top-to-bottom Bash script. Keep small shell functions only where they make cleanup, validation, or request construction easier to read. Use temporary files for the current snapshot, candidate map, request, and response. Make every `jeq ask --request -` invocation visible in the script.

Each cycle exposes one Choice question whose options are the current safe observed links plus `DONE` and `BLOCKED`. Jev therefore predicts the next click directly. The script executes only the opaque candidate ID mapped to a ref from that same observation.

## Acceptance criteria
- [ ] Replace the Hacker News path with one executable Bash script; it does not import `agent.py`, `policy.py`, `runner.py`, or any jeq library.
- [ ] The script starts at `https://news.ycombinator.com/`, not a dated page or discussion URL.
- [ ] The script passes the unchanged user goal, current UTC date, bounded current URL/title/visible text, recent decisions, and safe observed link choices to `jeq ask` on every step.
- [ ] Jev chooses one current candidate, `DONE`, or `BLOCKED`; code does not choose the route, preselect names based on the goal, calculate the maximum before the decision, or supply an expected item ID.
- [ ] A successful trace shows Jev choosing `past`, yesterday's date, the highest-comment discussion, and `DONE` across separate observe-decide-click cycles.
- [ ] The candidate map comes only from the latest Playwright snapshot and uses opaque script-owned IDs; model output never becomes a selector, URL, JavaScript, or shell fragment.
- [ ] Safety filtering is goal-independent: HTTPS and exact `news.ycombinator.com` host, broad read-only GET pages, and explicit rejection of external URLs, credentials, login, submit, vote, hide, reply, and other mutations.
- [ ] The script has fixed bounds for steps, candidates, visible text, request size, subprocess time, and comment evidence.
- [ ] Independent verification runs only after Jev's navigation decision and proves the chosen date and story had the largest displayed comment count; verification data never enters a Jev request.
- [ ] The script extracts a bounded set of top-level comments only after verification.
- [ ] Cleanup closes the named browser and removes temporary files on success, failure, signal, and timeout.
- [ ] A fake `jeq` plus fake `playwright-cli` test proves the full route without network or provider spend and rejects stale, unknown, external, and mutating choices.
- [ ] Remove or clearly retire the SDK-shaped Python browser framework and its callback-oriented documentation once equivalent script coverage exists.
- [ ] One user-authorized paid headed run passes from the Hacker News homepage; record each predicted click, model, requests, tokens, elapsed time, final story, verification, and cleanup.
- [ ] Fresh watcher passes at clean committed HEAD.

## Non-goals
- A reusable browser-agent SDK or Python package.
- General website support.
- Arbitrary form filling, login, voting, posting, or external article browsing.
- Hiding the CLI request behind helper objects or framework abstractions.

## Constraints
- Page content is untrusted state, not instructions.
- Security policy may remove unsafe actions, but it must not remove safe alternatives because they do not match the goal.
- Verification may judge the result, but it must not steer the model.
- Keep the script readable enough to teach the jeq CLI pattern without opening implementation modules.

