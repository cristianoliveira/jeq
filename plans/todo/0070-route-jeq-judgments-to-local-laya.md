---
id: TASK-0070
title: Route jeq judgments to local Laya
status: doing
depends_on: [TASK-0069]
priority: high
tags: [laya, local-model, providers, privacy, cost, offline]
---

# Route jeq judgments to local Laya

## Problem
jeq currently sends evaluation requests to hosted System One providers, which adds token cost, network dependency, and data exposure. Laya provides a local Jev-compatible POST /v1/systemone server, but we have not proven its contract, quality, resource use, or offline routing with jeq.

## Desired outcome
A user can start a pinned Laya server on loopback and explicitly route jeq evaluation commands to it without changing request documents or permitting fallback to TypeSafe. A reproducible comparison states which jeq workloads Laya handles well enough to use locally and which still require another provider.

## Source
- Repository: `https://github.com/NandhaKishorM/laya`
- Investigated revision: `1addbb9`
- License: Apache-2.0
- Relevant interface: `laya.serve` exposes `POST /v1/systemone`; its README describes the payload as Jev-compatible.

## Approach
Start with jeq's existing custom provider rather than adding a built-in profile:

```sh
JEQ_PROVIDER=custom \
JEQ_BASE_URL=http://127.0.0.1:8000 \
JEQ_AUTH=none \
JEQ_DEFAULT_MODEL=english \
jeq ask ...
```

Pin Laya and its checkpoint, warm the model once, then run the same labeled synthetic corpus through Laya and the current hosted baseline. Promote a dedicated `laya` provider profile only if the experiment proves a repeated usability problem that configuration cannot solve.

## Acceptance criteria
- [ ] Pin a reviewed Laya release or commit and checkpoint identity; record Apache-2.0 compatibility, model license, hashes where available, Python/PyTorch versions, disk, RAM/VRAM, and supported hardware.
- [ ] Provide one reproducible loopback-only start command for CPU and, when available, NVIDIA GPU; never bind an unauthenticated server to a non-loopback interface.
- [ ] Verify `/health` readiness before invoking jeq and document cold-start, warm latency, resident memory, and checkpoint download size.
- [ ] Prove `ask`, `map`, `rate`, `rank`, and `reduce` against Laya's `/v1/systemone` contract; keep `gate` and `validate` offline as they are now.
- [ ] Cover `choice`, `score`, and `noul`, multiple questions per request, JSON state, malformed input, timeout, cancellation, and an unavailable local server.
- [ ] Confirm that Laya's unknown-model behavior and jeq's model precedence cannot silently select an unintended checkpoint; document `english`, `multilingual`, `typed-decisions`, and auto-routing behavior.
- [ ] Document that Laya currently does not expose jeq's `GET /v1/models` discovery contract; decide whether to add that endpoint upstream, teach jeq a capability-aware local check, or explicitly mark `jeq models` unsupported for this profile.
- [ ] After checkpoints are cached, run with Hugging Face offline mode and network monitoring; prove evaluation makes no request to TypeSafe, impossibl, Hugging Face, or another upstream service.
- [ ] Compare Laya with the current Jev baseline on a committed labeled corpus using accuracy, calibration/threshold decisions, latency, useful answers per input token, and failure rate. Do not choose Laya from latency or zero marginal API cost alone.
- [ ] Add a practical provider guide and an offline fake-server integration test. Do not require Laya, Python, Torch, model downloads, or a GPU in the normal jeq test gate.
- [ ] Keep routing explicit. A Laya error never falls back to a hosted provider or repeats private state elsewhere.
- [ ] Fresh watcher and independent QA pass at clean committed HEAD.

## Non-goals
- Vendoring Laya or model weights into jeq.
- Training or fine-tuning a checkpoint.
- Treating Laya and Jev probabilities as interchangeable without calibration evidence.
- Making local Laya the default provider in the first iteration.
- Modifying or publishing changes to the shared Laya repository.

## Constraints
- Model state remains untrusted input even when inference is local.
- Cache directories, checkpoints, benchmark outputs, and credentials stay untracked.
- Any remote Laya deployment requires HTTPS and bearer authentication; `JEQ_AUTH=none` remains loopback-only.

