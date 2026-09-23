---
id: TASK-0070
title: Route jeq judgments to local Laya
status: done
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
- [x] Pin a reviewed Laya release or commit and checkpoint identity; record Apache-2.0 compatibility, model license, hashes where available, Python/PyTorch versions, disk, RAM/VRAM, and supported hardware.
- [x] Provide one reproducible loopback-only start command for CPU and, when available, NVIDIA GPU; never bind an unauthenticated server to a non-loopback interface.
- [x] Verify `/health` readiness before invoking jeq and document cold-start, warm latency, resident memory, and checkpoint download size.
- [x] Prove `ask`, `map`, `rate`, `rank`, and `reduce` against Laya's `/v1/systemone` contract; keep `gate` and `validate` offline as they are now.
- [x] Cover `choice`, `score`, and `noul`, multiple questions per request, JSON state, malformed input, timeout, cancellation, and an unavailable local server.
- [x] Confirm that Laya's unknown-model behavior and jeq's model precedence cannot silently select an unintended checkpoint; document `english`, `multilingual`, `typed-decisions`, and auto-routing behavior.
- [x] Document that Laya currently does not expose jeq's `GET /v1/models` discovery contract; decide whether to add that endpoint upstream, teach jeq a capability-aware local check, or explicitly mark `jeq models` unsupported for this profile.
- [x] After checkpoints are cached, run with Hugging Face offline mode and network monitoring; prove evaluation makes no request to TypeSafe, impossibl, Hugging Face, or another upstream service.
- [x] Compare Laya with the current Jev baseline on a committed labeled corpus using accuracy, calibration/threshold decisions, latency, useful answers per input token, and failure rate. Do not choose Laya from latency or zero marginal API cost alone.
- [x] Add a practical provider guide and an offline fake-server integration test. Do not require Laya, Python, Torch, model downloads, or a GPU in the normal jeq test gate.
- [x] Keep routing explicit. A Laya error never falls back to a hosted provider or repeats private state elsewhere.
- [x] Fresh watcher and independent QA pass at clean committed HEAD.

## Progress evidence
- Laya code `1addbb9ab8ffcd5b72a82d5158875f981384b7b8` and the English model are Apache-2.0. The local run used Python 3.12.12, PyTorch 2.7.0, Apple M4 Pro CPU, 51.5 GB RAM, no NVIDIA GPU, and 19 GB free disk before the checkpoint download. Laya's CPU guide requires 8 GB RAM and 10 GB disk; upstream states CUDA VRAM depends on checkpoint, batch size, and input length.
- Pinned English checkpoint `5e7b2b1b8ca2ecdd3f2322d94069c9b6ce7e844b`: 846 MB logical download, 2.72 GB maximum RSS, 5.1–8.4 s load, 153–175 ms warm direct inference on four CPU threads.
- Actual offline Laya HTTP run passed `ask`, `map`, `rate`, `rank`, and `reduce`; `validate` and `gate` passed with the server stopped.
- Network monitoring showed only the loopback listener while Hugging Face and Transformers offline modes were set.
- `jeq models` returns explicit HTTP 404 because Laya does not implement model discovery. The provider guide marks it unsupported.
- The 19-case Laya result has 68.4% overall acceptance by the committed primitive-specific predicates: classification 100%, Noul sign 100%, Score band 33.3%, and ranking top-1 33.3%.
- The authorized 19-request Jev 1.13.0 baseline also has 68.4% overall acceptance, but Noul band and ranking top-1 are both 100%. The committed evaluator reports the useful-answer metric and makes it `null` when usage is incomplete. Laya is 3.3 times faster and about 5.9 times more input-token efficient by that metric. Both providers are weak at explicit arithmetic and exact numeric ordering.
- Independent QA passed at `cd6611e`; fresh watcher generation 798 passed the full project gate with current worktree freshness.
- Local evidence report: `.tmp/reports/23-09-26/task-0070-laya-spike.md`.

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

