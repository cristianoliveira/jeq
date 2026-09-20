# Development

## Start safely

Use the Nix shell for the pinned toolchain:

```sh
nix develop
```

Do not put `TYPESAFE_API_KEY` in source, fixtures, command arguments, shell
history, or reports. Export it only in the process that needs an opt-in live
run. Use synthetic state and a pinned model for live checks. Never record raw
live payloads in Git.

## Test-driven workflow

1. Write a failing unit or contract test for the behavior.
2. Keep domain tests pure and deterministic.
3. Use `httptest.Server` for HTTP behavior. Record requests and use injected
   clocks or zero-delay retry headers instead of waiting.
4. Use fake readers, clients, and renderers for CLI unit tests.
5. Use `internal/blackbox` when the compiled binary, process streams, signals,
   or exit codes are part of the behavior. The suite builds the binary once and
   never calls `cli.Run`.
6. Run the focused package test, then the normal gate:

```sh
go test ./path/to/package -run TestName -count=1
nix develop -c make check
```

`make check` is the normal gate. It runs Nix formatting and flake checks,
Funzzy checks, a full build, `golangci-lint`, and all Go tests. A successful
`make check` prints only `true`.

CI and the normal gate must stay offline and deterministic. Do not add paid or
production-network tests to either one.

## Fake versus paid live tests

Use fakes by default:

- local `httptest.Server` covers status, retry, malformed response, timeout,
  and connection-drop behavior;
- temporary files and pipes cover source modes; and
- subprocess tests cover the compiled binary contract.

Paid live verification is opt-in and must run in an isolated harness outside
CI. Before starting, confirm the exact commit, endpoint, model pin, expected
cost, and key source. Read `TYPESAFE_API_KEY` from the environment inside the
harness. Do not print the key, put it in argv, persist raw responses, or run
unbounded retries. Prefer the smallest synthetic request that exercises the
contract. Record only sanitized schema and accounting evidence under the local
`.tmp` directory.

Repeat paid verification after any change to `cmd/`, `internal/`, `go.mod`,
`go.sum`, or `flake.nix`. TASK-0019 evidence is valid only for its exact
candidate commit.

## Build and release

Check the tree before release:

```sh
git status --short
nix develop -c make check
nix develop -c govulncheck ./...
go mod verify
go list -m all
```

Audit the module graph, `go.sum`, and the license file for every dependency.
The current graph has pinned Cobra, pflag, mousetrap, go-md2man, blackfriday,
YAML, and check modules. Their cached license files are MIT, BSD, or Apache
compatible with this project; no TOON module is present.

Verify a reproducible release candidate with GoReleaser. It owns the four
CGO-free targets, archives, README inclusion, and checksum file:

```sh
nix develop -c goreleaser check
rm -rf dist
goreleaser release --snapshot --clean
find dist -maxdepth 1 -type f -print
cat dist/checksums.txt
```

Inspect all four target archives and checksums. Extract the host-compatible
archive and run `jeq version`; it must report the snapshot version and commit
injected by GoReleaser. Snapshot builds never publish. To publish, commit and
push an approved `v*` tag; the tag workflow runs the normal gate, then lets
GoReleaser create one GitHub release. Tags such as `v0.1.0-rc.1` are published
as prereleases. Keep `dist` outside the repository or remove it before
committing.


Run the final gate from the clean committed tree. Inspect tracked files and the
history tip for credentials, raw live payloads, debug prints, stack traces, and
`.tmp` files. Do not tag or push until the board dependencies and paid live
verification are complete.
