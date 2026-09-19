# Release readiness

```text
evidence -> local blockers (0 calls) -> risk/manual/rollout judgment -> policy receipt
```

```sh
python3 examples/python/release_readiness.py < examples/python/fixtures/release.json
```

Failed tests or known vulnerabilities block with exit `10` and no model call.
Otherwise one call costs one evaluation. The demonstration policy uses risk
`<=0.30` for pass with full rollout, risk `<=0.60` for canary, risk/manual
`>=0.80` or `hold` for block, and exit `11` for low-confidence evidence. These
values are not calibrated release controls. Receipts retain typed answers,
thresholds, usage, model, and stage count, not raw evidence.
