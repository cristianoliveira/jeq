# examples

## jeq browser example

`examples/playwright-browser-agent/hn-browser.sh` demonstrates direct Unix composition: `playwright-cli` observes Hacker News, `jq` builds a finite Choice request, and `jeq ask --request -` predicts the next safe opaque link. The script starts at the HN homepage, keeps navigation read-only and same-host, bounds subprocesses and evidence, and verifies the final date, maximum discussion count, and comments independently.

Run the offline fake-CLI acceptance test:

```sh
examples/playwright-browser-agent/test-hn.sh
```

For an explicitly authorized paid headed run, configure `jeq` and run:

```sh
JEQ_GOAL='navigate to yesterday then the most-discussed discussion' examples/playwright-browser-agent/hn-browser.sh --headed
```

## Laya provider evaluation

`examples/laya-evaluation/` contains a bounded synthetic corpus for comparing local Laya with another System One provider. It reports transport failures separately from Choice, Noul, Score, ranking, and reduction quality.

Test the evaluation harness without network or provider calls:

```sh
examples/laya-evaluation/test.sh
```

For the opt-in test that starts a pinned real Laya checkpoint and sends 19 actual local judgments, follow `examples/laya-evaluation/README.md`. The real test is not part of the normal CI gate because it requires Laya, Torch, and model weights.
