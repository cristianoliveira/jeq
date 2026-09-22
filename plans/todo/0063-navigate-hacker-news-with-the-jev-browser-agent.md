---
id: TASK-0063
title: Navigate Hacker News with the Jev browser agent
status: doing
depends_on: [TASK-0062]
priority: high
tags: [examples, jev, playwright, hacker-news, security, paid-test]
---

# Navigate Hacker News with the Jev browser agent

## Problem
The current Jev browser showcase proves only a loopback fixture. We need a bounded real-site trial that lets Jev navigate Hacker News to yesterday's most-discussed story and its discussion without permitting external navigation, mutations, login, voting, or arbitrary extraction.

## Desired outcome
A headed, paid Jev run starts on Hacker News's previous-day front page, observes only read-only same-host navigation candidates, selects the discussion link with the largest displayed comment count, opens that discussion, and selects DONE from visible evidence. Code independently proves the selected item was the maximum and extracts a bounded set of top-level comments for review.

## Acceptance criteria
- [ ] Add an explicit HTTPS host allowlist to the browser boundary; loopback behavior remains unchanged.
- [ ] Remote read-only mode exposes only same-host navigation links and fixed scroll/wait controls.
- [ ] Reject credentials, non-HTTPS remote URLs, external links, login, submit, vote, hide, and other mutating paths before navigation.
- [ ] Parse observed link destinations into typed elements; model output still selects opaque current IDs, never URLs.
- [ ] Start at `https://news.ycombinator.com/front?day=<yesterday>` and state the exact date in the goal.
- [ ] Before Jev acts, independently derive the maximum displayed comment count and expected item URL from observed candidates.
- [ ] Jev selects the discussion link in one bounded request, then selects DONE from bounded visible discussion evidence.
- [ ] Independent verification checks same host, expected item ID, discussion title/body, and at least one top-level comment.
- [ ] Extract and report a bounded set of top-level comment texts without sending credentials, cookies, or hidden DOM state.
- [ ] Unit tests cover hostile/external/mutating links, same-host relative URLs, maximum selection verification, and cleanup.
- [ ] Offline fake-provider test passes before any paid request.
- [ ] One user-authorized paid smoke passes in a headed browser; record model, request count, tokens, elapsed time, selected story, and cleanup.
- [ ] Fresh watcher passes at clean committed HEAD.

## Constraints
- No login, voting, hiding, submission, form filling, or external article navigation.
- Hacker News page text is untrusted state and cannot change the goal or action policy.
- Do not claim general website support from this single read-only trial.

