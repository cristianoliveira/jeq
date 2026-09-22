# Playwright browser boundary

This local example observes a page through the installed `playwright-cli`, converts only current accessibility refs into typed allowlisted actions, and inspects the outcome independently. It does not call Jev or import browser libraries.

Run offline tests:

```sh
python3 examples/playwright-browser-agent/test_agent.py
```

Run the local smoke flow (requires `playwright-cli` on `PATH`):

```sh
python3 examples/playwright-browser-agent/smoke.py
```

The smoke flow starts a loopback fixture, opens a unique named session, fills the observed search field, checks the independent title and URL, and always closes the session. Browser output belongs under `output/playwright/playwright-browser-agent/` when captured.
