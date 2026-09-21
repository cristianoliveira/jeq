---
id: TASK-0037
title: Benchmark jeq analysis skill with DeepSeek
status: doing
depends_on: []
priority: high
tags: []
---

# Benchmark jeq analysis skill with DeepSeek

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
- jeq budget: exactly 16 maximum HTTP attempts across all runs; `--max-retries 0`
- Iterations: one; analyze results before deciding whether to revise

## Acceptance criteria
- [x] Freeze prompts, fixtures, expectations, model, and settings before paired runs.
- [x] Candidate and baseline use identical files and DeepSeek settings in fresh run directories.
- [x] Runtime failures are reported as errors rather than graded as passes or negatives.
- [ ] Grade every declared expectation with transcript/output evidence and state grader independence limits.
- [x] Aggregate pass rate, duration, and authoritative token usage; identify non-discriminating assertions and limitations of one repetition.
- [x] Generate the standard static review report and preserve all raw artifacts outside the skill directory.
- [x] Live jeq calls use only synthetic fixtures, read the existing credential from environment, never print/copy it, disable retries, and stay within 16 total HTTP attempts.

## Iteration 1 result
Four of six runs completed. The eval 2 and eval 3 no-skill baselines timed out at 300 seconds and remain ungraded. Candidate completed 15/16 expectations; the only completed pair favored the skill 7/7 versus 6/7. A second approved paid iteration is required for a complete paired benchmark.

## Iteration 2 result — partial, 2026-09-21
The frozen prompts, fixtures, model, medium thinking, sequential execution, and 300-second timeout were reused. Four of six runs completed: both candidate content runs and both deterministic-latency runs. The eval 1 and eval 2 no-skill baselines timed out at 300 seconds while exploring the local environment; they produced no valid deliverable and remain ungraded. Candidate eval 1 and eval 2 each recorded exactly four HTTP attempts with `--max-retries 0`; both eval 3 runs made no JEQ call. Observed TypeSafe attempts: 8 of the 16-attempt ceiling; no credential or raw payload was printed. This iteration remains incomplete because the two paired no-skill content baselines failed at runtime. Raw artifacts are under `/Users/cristianoliveira/.agents/jeq-complex-analysis-workspace/iteration-2/`; see `.tmp/reports/21-09-26/TASK-0037-second-iteration.md`.

