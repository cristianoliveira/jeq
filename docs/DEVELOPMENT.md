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

Build reproducible, CGO-free release candidates for every supported target:

```sh
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go build -trimpath \
      -ldflags "-X github.com/cristianoliveira/gev/internal/cli.Version=$VERSION \
                -X github.com/cristianoliveira/gev/internal/cli.Commit=$COMMIT" \
      -o "dist/gev-${target%/*}-${target#*/}" ./cmd/gev
done
```

Confirm every artifact is non-empty. Run the host-compatible artifact with
`gev version` and verify the injected version and commit. Keep artifacts outside
the repository or remove them before committing.

Run the final gate from the clean committed tree. Inspect tracked files and the
history tip for credentials, raw live payloads, debug prints, stack traces, and
`.tmp` files. Do not tag or push until the board dependencies and paid live
verification are complete.
