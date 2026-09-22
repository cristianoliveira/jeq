# Jev-directed Playwright browser

This example observes a local page with `playwright-cli`, asks Jev to choose one finite next action through `jeq ask`, executes only a current observed ref, and verifies the final page independently.

It reproduces the core pattern from the MIT-licensed [jev-ultrafast](https://github.com/browser-use/jev-ultrafast) project:

1. Observe the current page.
2. Build a finite operation and target vocabulary.
3. Ask all independent Choice questions in one System One request.
4. Consume only the target head that matches the selected operation.
5. Execute one allowlisted action and observe again.

This version uses `playwright-cli` instead of Browser Harness. It uses caller-provided fill values instead of a separate text-generation model. It is a bounded local demonstration, not evidence of general browser-task reliability or of the source project's latency claims.

## Safety model

- The browser opens only loopback HTTP URLs without credentials.
- Jev selects opaque code-owned candidate IDs. Its output never becomes a selector, JavaScript expression, shell command, or Playwright command name.
- Fill text and dropdown options come from finite caller or page allowlists.
- Every decision is mapped back to the latest in-memory action plan and then checked again by the browser boundary.
- Mutations are never retried automatically.
- Step, state, criteria, visible body text, command, and subprocess timeouts are bounded.
- Jev receives bounded visible body text so completion decisions use actual page evidence.
- `DONE` succeeds only when the final URL, title, and body satisfy fixture-owned checks.
- The browser session and fixture server close on every exit path.

## Test offline

Run unit tests:

```sh
python3 examples/playwright-browser-agent/test_agent.py
python3 examples/playwright-browser-agent/test_policy.py
python3 examples/playwright-browser-agent/test_runner.py
```

Run the full local flow with the real installed `jeq` and `playwright-cli` binaries. The TypeSafe API is replaced by a loopback fake, so this spends no provider budget:

```sh
python3 examples/playwright-browser-agent/fake_e2e.py
```

The fake service returns typed Choice envelopes for the local test sequence. Production policy and runner code contain no fixture-specific action sequence.

## Run with Jev

Set the credential for your selected jeq provider, then run:

```sh
export TYPESAFE_API_KEY='...'
python3 examples/playwright-browser-agent/demo.py --headed
```

Omit `--headed` in environments without a display.

The default goal is:

> Find Design stays in Lisbon with free cancellation, then open Casa Flora.

The command starts the fixture on a random loopback port. Each cycle makes one paid `jeq ask` request. It prints one compact JSON trace per decision and a final PASS record with total decisions, token usage, elapsed time, model versions, and screenshot path. It does not print credentials or raw request state.

To change the finite text vocabulary, repeat `--fill-value`:

```sh
python3 examples/playwright-browser-agent/demo.py \
  --goal 'Find Design stays in Lisbon with free cancellation, then open Casa Flora.' \
  --fill-value Lisbon
```

Artifacts are written under `output/playwright/playwright-browser-agent/`. The default screenshot is `final.png`.

## Files

- `agent.py`: Playwright CLI observation and allowlisted execution boundary.
- `policy.py`: native request builder, Jev subprocess adapter, and typed decision consumer.
- `runner.py`: bounded observe, decide, execute, and verify loop.
- `demo.py`: loopback fixture server and live Jev entry point.
- `fake_e2e.py`: real CLI end-to-end test with a fake loopback TypeSafe API.
- `fixture.html`: deterministic local page.
