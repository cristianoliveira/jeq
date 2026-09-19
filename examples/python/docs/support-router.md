# Support router

```text
ticket -> one Choice/Noul judgment -> confidence policy -> fixed queue label
```

Run one call:

```sh
python3 examples/python/support_router.py < examples/support-routing/fixtures/ticket.txt
```

Route confidence below `0.70`, or a category outside `billing`, `technical`,
and `sales`, returns `human_review` with exit `0`. The receipt keeps the typed
answers, model, usage, and `stage_count: 1`; it never includes the ticket.
The threshold is a demonstration, not a calibrated support policy. One live run
costs one model evaluation.
