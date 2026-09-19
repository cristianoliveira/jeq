# Support routing

This workflow reads one ticket from stdin and asks three independent questions
in one `gev ask` request: an allowlisted route, urgency, and escalation. The
script maps only `billing`, `technical`, and `sales` to fixed queue labels.
Unknown routes and route confidence below `0.70` become `human_review`.

```sh
GEV_BIN=gev GEV_MODEL=jev-latest \
  ./route.sh < fixtures/ticket.txt
```

Set `GEV_BASE_URL` to point at a local fake server or an explicitly approved
TypeSafe endpoint. The API key remains in the environment and is never read by
this script. A successful call emits one JSON receipt. A gev exit `1`, `2`, or
`130` is forwarded with gev's structured output unchanged.

The confidence threshold is a demonstration policy, not a production
calibration. Keep the allowlist and action labels in deterministic code; never
turn model output into a command or shell fragment.
