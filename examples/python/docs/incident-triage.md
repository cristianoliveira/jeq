# Incident triage

```text
stage 1: category/severity/page -> confidence gate
                  |
                  v
stage 2: category-scoped Choice over versioned runbook IDs
```

```sh
python3 examples/python/incident_triage.py < examples/python/fixtures/incident.json
```

Uncertain stage-one evidence makes one call and returns `human_review`/`11`.
Confident evidence makes a second call whose criteria contain only IDs from the
selected category in `specs/incident-runbooks.json`. Severity Score is an ordered
level index `0..3`; it remains raw evidence and is not treated as a probability.
Invalid or low-confidence selection also returns `human_review`/`11`; no runbook is executed. A happy path
costs two evaluations and reports `stage_count: 2`, aggregated usage, and both
sets of typed answers. Thresholds are demonstrations, not on-call calibration.
