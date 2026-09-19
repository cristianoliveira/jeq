# Cobra root help verification

Date: 2026-09-19  
Task: TASK-0031  
Verdict: PASS

Bare `jeq` and `jeq --help` are byte-for-byte identical standard Cobra help (805 bytes), exit 0, and write no stderr. The result is unchanged with empty or unreadable home/config paths.

The root path performs no environment read, config/filesystem read, renderer call, client creation, or network request. The previous credential/model dashboard is removed; workflow discovery remains explicit through `jeq examples`.

Full tests, architecture rules, lint, build, Nix checks, govulncheck, and independent QA pass. TASK-0030 human/result output boundaries remain unchanged.
