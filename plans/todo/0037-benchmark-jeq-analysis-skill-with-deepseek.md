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
- TypeSafe: paid live service against synthetic fixtures; existing environment credential
- JEQ budget: exactly 16 maximum HTTP attempts across all runs; `--max-retries 0`
- Iterations: one; analyze results before deciding whether to revise

## Acceptance criteria
- [x] Freeze prompts, fixtures, expectations, model, and settings before paired runs.
- [x] Candidate and baseline use identical files and DeepSeek settings in fresh run directories.
- [x] Runtime failures are reported as errors rather than graded as passes or negatives.
- [ ] Grade every declared expectation with transcript/output evidence and state grader independence limits.
- [x] Aggregate pass rate, duration, and authoritative token usage; identify non-discriminating assertions and limitations of one repetition.
- [x] Generate the standard static review report and preserve all raw artifacts outside the skill directory.
- [x] Live JEQ calls use only synthetic fixtures, read the existing credential from environment, never print/copy it, disable retries, and stay within 16 total HTTP attempts.

## Iteration 1 result
Four of six runs completed. The eval 2 and eval 3 no-skill baselines timed out at 300 seconds and remain ungraded. Candidate completed 15/16 expectations; the only completed pair favored the skill 7/7 versus 6/7. A second approved paid iteration is required for a complete paired benchmark.

