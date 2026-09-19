# CLI example discovery verification

Date: 2026-09-19  
Task: TASK-0029  
Verdict: PASS (0 blockers)

## Discovery contract

A fresh installed binary now reaches a full pipeline recipe in three offline commands:

1. `jeq`
2. `jeq examples`
3. `jeq examples map-reduce-gate`

Measured from an empty temporary home and working directory:

- home output: 277 bytes
- catalog output: 965 bytes
- network calls during discovery: 0
- catalog IDs: `validate-native`, `ask-native`, `map-gate`, `reduce-gate`, `map-reduce-gate`

Catalog and details require no API key, config, checkout files, jq, stdin, or network access. Unknown IDs and extra arguments return one structured `JEQ_INPUT_INVALID` document, exit 2, and no stderr.

## Recipe verification

All five embedded shells pass `bash -n`. `validate-native` runs offline from an empty directory. Network recipes ran against a local fake endpoint:

| Recipe | Expected calls | Observed calls |
| --- | ---: | ---: |
| `ask-native` | 1 | 1 |
| `map-gate` | N = 2 | 2 |
| `reduce-gate` | 1 | 1 |
| `map-reduce-gate` | N + 1 = 3 | 3 |

All completed successfully. Final pipeline outputs contained no source items, paths, or content. Gate policy exits remain 0/pass, 10/reject, and 11/uncertain through `pipefail`.

## Checks

- `nix develop -c make check`: PASS
- full Go tests, lint, vet, architecture rules: PASS
- `nix develop -c govulncheck ./...`: no vulnerabilities
- root and local command help examples: PASS
- independent QA: PASS

The intentional home `next_step` migration from `jeq ask --help` to `jeq examples` is documented and tested. Recipe outputs are transport/policy evidence only; no live model-quality assertion was made.
