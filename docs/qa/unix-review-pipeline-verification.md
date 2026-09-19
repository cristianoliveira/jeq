# Unix review pipeline verification

Date: 2026-09-19  
Task: TASK-0028  
Verdict: PASS (0 blockers)

## Deterministic contract

- The script visibly composes record emission, `jeq map`, `jq`, `jeq reduce`, `jeq gate`, and a safe final `jq` projection.
- All files are preflighted before the first API request.
- Two ordered fixture files produce exactly two map requests and one reduce request.
- Map receives exact `{path,content}` state. Reduce receives only `{file:{path,content},local_focus}` per item.
- Gate adds no request and preserves pass, reject, and uncertain exits 0, 10, and 11 through `pipefail`.
- Final stdout retains aggregate and gate evidence but removes source items, paths, and content.
- Tests assert transport and policy behavior only; they make no model-quality claim.

Checks:

- `nix develop -c make check`: PASS
- `nix develop -c govulncheck ./...`: no vulnerabilities
- Bash 5 and macOS Bash syntax: PASS
- `shfmt`: PASS
- Independent QA: PASS

## Bounded live observation

Model: `jev-1.13.0`

| Stage | Answer | Usage (input/output) |
| --- | ---: | ---: |
| Map: first fixture | .93 | 401 / 21 |
| Map: second fixture | .96 | 393 / 21 |
| Reduce: related collection | .93 | 496 / 21 |

Request count: three (two map plus one reduce). Gate decision: pass, exit 0. Gate and both jq stages made no requests. The temporary source-bearing observation was deleted; retained output is source-free.

The values are observations, not deterministic truth or calibrated review policy.
