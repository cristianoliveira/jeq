---
id: TASK-0037
title: Benchmark JEQ analysis skill with DeepSeek
status: doing
depends_on: []
priority: high
tags: []
---

# Benchmark JEQ analysis skill with DeepSeek

## Problem
The new skill is structurally valid, but we do not yet have measured evidence that it improves agent behavior over an unskilled baseline on its declared cases.

## Agreed benchmark
- Model: `deepseek/deepseek-v4-flash`, medium thinking
- Cases: all three declared content evaluations
- Comparison: candidate skill versus no-skill baseline
- Repetitions: one per condition (six agent runs)
- Concurrency: sequential
- Timeout: 300 seconds per run
- TypeSafe: loopback deterministic fake only; no live service calls
- Iterations: one; analyze results before deciding whether to revise

## Acceptance criteria
- [ ] Freeze prompts, fixtures, expectations, model, and settings before paired runs.
- [ ] Candidate and baseline use identical files and DeepSeek settings in fresh run directories.
- [ ] Runtime failures are reported as errors rather than graded as passes or negatives.
- [ ] Grade every declared expectation with transcript/output evidence and state grader independence limits.
- [ ] Aggregate pass rate, duration, and authoritative token usage; identify non-discriminating assertions and limitations of one repetition.
- [ ] Generate the standard static review report and preserve all raw artifacts outside the skill directory.
- [ ] No request reaches the real TypeSafe endpoint and no production credential is read or copied.

