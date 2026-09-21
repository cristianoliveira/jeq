# Change risk gate

This workflow reads one diff from stdin and applies an explicit demonstration
policy to the `safe_to_ship` Noul signal:

- `>= 0.80`: `pass`, exit `0`;
- `<= 0.30`: `review_or_block`, exit `10`; and
- between those thresholds: `uncertain`, exit `11`.

```sh
JEQ_MODEL=jev-latest \
  ./gate.sh < fixtures/change.diff
```

Policy exits are owned by this script. jeq operational, usage, and interruption
exits (`1`, `2`, and `130`) are returned unchanged with jeq's structured JSON
error document. Missing or malformed model evidence produces an `uncertain`
receipt and exit `11`. The example's `130` test uses jeq's already-black-boxed
interrupt contract plus a deterministic shim instead of a race-prone signal.

The thresholds are examples for workflow design, not calibrated release policy.
No deployment action is performed. Set `TYPESAFE_BASE_URL` for a local fake server
or an explicitly approved live endpoint; live calls consume the account's API
budget and are never part of CI.
