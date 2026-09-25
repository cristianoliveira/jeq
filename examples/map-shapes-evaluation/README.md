# jeq map request-shape evaluation

This folder builds and checks an offline plan for comparing three map request shapes. It does not call a provider, consume credentials, or write responses. It cannot establish which strategy is more accurate or cheaper.

## Run offline checks

Run the deterministic tests and inspect the proposed matrix:

```sh
python3 -m unittest discover -s examples/map-shapes-evaluation -p 'test_*.py' -v
python3 examples/map-shapes-evaluation/harness.py check --allow-shared-context
python3 examples/map-shapes-evaluation/harness.py plan --allow-shared-context
```

Shared state includes every record in one request. The flag is required even for these synthetic fixtures. Do not use real or private records unless the data owner explicitly authorizes that disclosure. The harness marks the plan offline-only, and its mapping tests prove answer IDs are routed to the right record in input order. They do not prove Jev will semantically ignore other records; the paid phase must measure that risk.

## Workloads and strategies

`fixtures.json` defines four records for each workload. Padding strings expand deterministically at load time. Expected labels stay in the fixture and are never copied into request bodies.

| Workload | What it checks |
| --- | --- |
| `short-unrelated` | Short records with different topics |
| `large-unrelated` | A relevant message surrounded by repeated irrelevant detail |
| `multi-question` | Three judgments per record and answer-ID routing |
| `exact-duplicates` | Two byte-identical request pairs; only these may deduplicate |
| `large-shared-reference` | One repeated synthetic policy document with four separate records |

The planner compares:

1. `per-record`: one request per record, using all configured questions.
2. `exact-dedup`: canonical JSON equality over model, state, and questions; no semantic similarity or text normalization.
3. `shared-context`: one structured state with namespaced per-record questions. The request names one record in each instruction and ignores other record entries.

The mapping test deliberately shuffles response-map order, checks every namespaced answer ID against its record and original question, then restores the original record order. An unknown answer ID fails closed. Exact dedup reuses a response only for records with identical canonical requests.

## Safety and privacy

The current Jev 1.13 documentation lists 64k total tokens per request and 32k tokens for `state` plus the longest question. The harness applies byte-only preflight ceilings of 24 KiB for the full serialized request and 12 KiB for serialized state plus the longest serialized question. That leaves at least 50% byte headroom against those published limits. Bytes are a safety check, not a tokenizer or billing estimate. Oversized plans fail; the harness does not truncate or retry them.

The synthetic shared reference and records are safe to send only because they are invented fixtures. A shared-context request exposes all included records together. Keep shared context disabled for private data unless its owner explicitly opts in. The harness has no API client or networking code. Paid caches, credentials, and results belong under ignored paths `private/`, `.cache/`, and `results/`; none are created by these offline commands.

## TypeSafe guidance

Recheck the live docs before authorizing a run because model limits and prices may change:

- [Current model limits and prices](https://docs.typesafe.ai/models.md): Jev 1.13 is `jev-1.13.0`, input is charged at $42 per billion tokens ($0.042 per million), output is free, and requests are limited to 64k total / 32k state plus longest question.
- [Jev 1.13 jaggedness](https://docs.typesafe.ai/model-jaggedness/jev-1.13.md): accuracy can fall as unrelated state grows; filter state before sending it. Multiple indirection levels can also weaken answers. The shared-context strategy adds both unrelated-record exposure and record lookup indirection, so a lower request count alone is not a reason to accept it.

## Paid results are separate

`harness.py check` runs fake usage and probability examples only to test metric arithmetic. The printed values are marked synthetic and are not TypeSafe measurements, accuracy claims, or savings claims. No response cache or paid output is committed.

The [proposed paid run matrix](paid-run-proposal.md) specifies calls, model, request/token planning guards, quality checks, a planning estimate, and a separate full-context list-price envelope. The 814,000-token figure is not a guaranteed billing cap. Stop here until Cristian separately authorizes the $0.36288 envelope and confirms the data is safe to send.
