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

## Compare a hosted provider

The script never selects a provider. Configure one explicitly, review its privacy and cost, and authorize all 19 requests before setting `JEQ_EVAL_CONFIRM=1`. Use the same `cases.ndjson` and save each result separately.

Do not send real customer data. The corpus is synthetic.

## Read the result

The JSON report includes every case and these aggregate signals:

- Exact accuracy for all primitives
- Choice accuracy and multiclass Brier score
- Noul sign accuracy at 0.5, strict acceptance-band rate, and Brier score
- Score acceptance-band rate and mean absolute error
- Ranking top-1 and exact-order accuracy
- Provider-reported token usage and observed latency

Transport failures are reported separately and excluded from quality denominators. Latency and zero output tokens do not count as judgment quality.

This corpus is deliberately small. It can find obvious incompatibility, inversion, and calibration problems. It cannot establish production accuracy. Add representative held-out cases for the actual domain before changing a provider default.
