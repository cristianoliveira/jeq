# JEQ rebrand verification

Date: 2026-09-19  
Tasks: TASK-0033 and TASK-0032  
Verdict: PASS

## Rebrand closure

- Go module: `github.com/cristianoliveira/jeq`
- Executable/package: `cmd/jeq`; no predecessor command target or alias
- Domain package: `internal/domain/jeq`
- Error namespace: `JEQ_*`
- Evidence envelope: `_jeq`
- Product shell variables: `JEQ_*`
- Configuration: `${XDG_CONFIG_HOME:-$HOME/.config}/jeq/config.json`
- Provider variables: `TYPESAFE_*` unchanged

Tracked paths contain no predecessor product name. Tracked content contains no standalone predecessor token, legacy error prefix, or legacy envelope key. Git history, unrelated identifiers such as `existingEvidence`, ignored caches, and the local checkout directory are outside this source-tree contract.

## Runtime verification

A clean built `jeq` binary passed root/help/completion and native Cobra example discovery. All five recipe commands are offline and use JEQ names. The predecessor build target fails and no compatibility binary is installed.

Isolated config checks prove the JEQ path is read and the predecessor path is ignored. Old product-owned variables have no effect. Fake endpoints verified:

- map writes `_jeq` and preserves record/response data;
- reduce makes one collection request and writes `_jeq`;
- gate reads the new envelope, makes no request, and preserves exits 0/10/11;
- predecessor-envelope-only input is not treated as evidence and fails with `JEQ_INPUT_INVALID`;
- auth and local failures use `JEQ_*`, correct exits, stderr-only errors, and no secret/cause leaks.

## Checks and external boundary

Full Go tests, vet, build/install, golangci-lint, Nix/Fzz checks, architecture rules, `govulncheck`, tracked-tree scans, fake-endpoint workflows, and independent QA pass.

`git remote -v` is empty. Renaming a hosted repository was therefore unavailable and remains an external action; the local module is ready for `github.com/cristianoliveira/jeq`.
