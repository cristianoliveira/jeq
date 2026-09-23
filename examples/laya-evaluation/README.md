# Compare Laya with another System One provider

This bounded corpus measures judgment quality separately from transport compatibility. It contains 19 synthetic cases for Choice, Noul, Score, ranking, and collection reduction. Expected answers never enter provider requests.

## Run against local Laya

Start Laya on loopback with one checkpoint:

```sh
LAYA_HOST=127.0.0.1 LAYA_DEVICE=cpu LAYA_MODELS=english laya-serve
```

Check the request count, then run:

```sh
JEQ_PROVIDER=custom \
JEQ_BASE_URL=http://127.0.0.1:8000 \
JEQ_AUTH=none \
JEQ_EVAL_MODEL=english \
JEQ_EVAL_CONFIRM=1 \
./examples/laya-evaluation/run.sh > laya-results.json
```

The first server start downloads model weights. Pin and review the Laya code and checkpoint revisions before comparing results. Use `HF_HUB_OFFLINE=1` after the cache is warm to prove inference does not depend on Hugging Face.

## Run the real acceptance test

The normal Go gate uses a controlled HTTP server because CI must not download a checkpoint. The opt-in acceptance test is different: it starts the real pinned Laya `Agent`, `Router`, and FastAPI server, then sends all 19 judgments through the real `jeq` binary. It enables Hugging Face offline mode and accepts only a loopback listener.

Install Nix, `jq`, `curl`, `lsof`, `pgrep`, and `shasum`. Clone Laya at the pinned source revision, keep that checkout clean, and download the pinned English checkpoint once. Then run:

```sh
JEQ_REAL_LAYA_CONFIRM=1 \
LAYA_CHECKOUT=/path/to/laya \
LAYA_MODEL_PATH=/path/to/models--convaiinnovations--laya/snapshots/5e7b2b1b8ca2ecdd3f2322d94069c9b6ce7e844b \
./examples/laya-evaluation/test-real-laya.sh
```

The test verifies the clean Laya commit and SHA-256 hashes for the model weights and required configuration files. It builds jeq from the current checkout into a fresh temporary directory. It refuses an occupied port and incomplete or malformed responses. It does not use a fake provider or mocked model output. It prints the measured summary as JSON, stops the spawned listener, verifies the port is free, and then removes temporary files.

## Compare a hosted provider

The evaluator never selects a provider. Configure one explicitly, review its privacy and cost, and authorize all 19 requests before setting `JEQ_EVAL_CONFIRM=1`. Use the same `cases.ndjson` and save each result separately.

Do not send real customer data. The corpus is synthetic.

## Read the result

The JSON report includes every case and these aggregate signals:

- Overall acceptance rate: exact Choice label, Noul direction at 0.5, in-band Score, or exact ranking order
- Choice accuracy and multiclass Brier score
- Noul sign accuracy at 0.5, strict acceptance-band rate, and Brier score
- Score acceptance-band rate and mean absolute error
- Ranking top-1 and exact-order accuracy
- Useful answers: accepted Choice, Noul, or Score results plus correct ranking top-1
- Useful answers per provider-reported input token
- Provider-reported token usage and observed latency

Transport failures, invalid responses, and unscorable responses are reported separately and excluded from quality denominators. A Choice Brier score requires complete criteria support, values in `[0,1]`, and probabilities that sum to 1 within 0.001. A rank result requires the same probability checks. Incomplete Choice probabilities leave the label scorable but mark its probability metric unscorable.

Token efficiency uses all requests with reported usage, including invalid or unscorable answers. It is `null` unless every attempted request reports usage because a failed request can still incur unknown billed tokens. Latency and zero output tokens do not count as judgment quality.

This corpus is deliberately small. It can find obvious incompatibility, inversion, and calibration problems. It cannot establish production accuracy. Add representative held-out cases for the actual domain before changing a provider default.
