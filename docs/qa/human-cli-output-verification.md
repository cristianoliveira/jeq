# Human CLI output verification

Date: 2026-09-19  
Task: TASK-0030  
Verdict: PASS

## Boundary

Human interactions are plain text:

- home and examples
- version and models
- validation receipts
- Cobra help, usage failures, and operational errors

Evaluation results remain lossless JSON/NDJSON:

- ask
- map
- reduce
- gate

The JSON renderer exposes only successful result methods. There is no global output-format flag and no JSON error renderer.

## Binary evidence

From an empty home/config environment:

- home, examples, version, and validate wrote deterministic plain stdout and empty stderr;
- models used a fake endpoint and printed `name: description (release_date)`, omitting unknown response fields;
- help used standard Cobra prose on stdout;
- unknown `--output` exited 2 with `Error: unknown flag: --output` on stderr and empty stdout;
- misspelled commands used Cobra's native suggestion;
- coded failures used `Error: JEQ_*` on stderr and did not expose wrapped causes or server-supplied secrets;
- gate pass/reject/uncertain kept exits 0/10/11, JSON output, and empty stderr;
- ask/map/reduce fake-endpoint successes remained parseable, lossless JSON/NDJSON.

## Checks

- `nix develop -c make check`: PASS
- full Go tests, lint, vet, build, architecture rules: PASS
- `nix develop -c govulncheck ./...`: no vulnerabilities
- `git diff --check`: PASS
- independent QA and blocker recheck: PASS

Historical QA reports still describe the contracts they verified at that time. Current ADR 0001, ADR 0003 status, and `docs/ARCHITECTURE.md` describe the new boundary.
