# Code-smell question verification

Date: 2026-09-19  
Task: TASK-0027  
Verdict: PASS (0 blockers)

## Deterministic contract

- The example sends one request with five independent positive Noul questions.
- The request preserves ordered `{path,content}` items and treats source, comments, and strings as untrusted data.
- Each question has aligned `true` and `false` boundaries. Policy and abstraction questions define the no-applicable-case as true.
- The safe projection removes top-level `.items`, sorts dimensions from lowest probability to highest, and computes `quality_floor` as their minimum.
- Offline gate tests cover pass, uncertain, and reject decisions with exits 0, 11, and 10. The gate makes no API request.
- Fake-response tests make no claim about model quality.

Checks:

- `nix develop -c make check`: PASS
- `nix develop -c govulncheck ./...`: no vulnerabilities
- Bash and macOS `/bin/bash` syntax checks: PASS
- Independent QA: PASS

## Bounded live observations

Model: `jev-1.13.0`

| Input | Responsibilities | Policy | Dependencies | Abstractions | Complexity | Floor | Usage (in/out) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Cohesive fixture | .87 | .93 | .97 | .63 | .81 | .63 | 923 / 107 |
| Duplicated-policy fixture | .70 | .06 | .94 | .56 | .11 | .06 | 929 / 107 |
| Related `map.go` + `reduce.go` set | .33 | .24 | .47 | .40 | .24 | .24 | 6326 / 107 |

The intended policy dimension separated by `.87` between controlled fixtures. This is more diagnostic than the previous forced Choice confidence of `.25`, because compatible weaknesses no longer compete.

These values are observations, not deterministic truth, calibrated thresholds, or CI assertions. The moderate abstraction value for the small cohesive fixture remains a known model/context limitation.
