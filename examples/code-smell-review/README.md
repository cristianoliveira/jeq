# Probabilistic code-smell review

`review.sh` sends an explicitly selected, bounded set of source files through one
`gev reduce` request. It adds probabilistic design evidence that complements
compiler errors, tests, linters, and static analysis; it is not a linter, proof,
a refactoring tool, or a replacement for deterministic checks.

The script reads only the paths passed as arguments, preserves their order, and
rejects missing, unreadable, non-regular, oversized, or more than 20 files. Each
file is limited to 256 KiB. The request embeds one strict question document and
uses the configured model. A successful review is advisory and exits 0.

## Run it

```sh
GEV_BIN=gev JQ_BIN=jq \
  ./examples/code-smell-review/review.sh \
  internal/cli/map.go internal/domain/pipeline/reduce.go
```

For changed files, use NUL-delimited Git output so spaces and newlines in paths
are not split by the shell:

```sh
mapfile -d '' changed < <(
  git diff --name-only -z --diff-filter=ACMR -- '*.go' '*.sh'
)
if ((${#changed[@]})); then
  ./examples/code-smell-review/review.sh "${changed[@]}"
fi
```

The review sends source code to the configured TypeSafe endpoint and spends one
API request per invocation. Do not send credentials, secrets, generated private
data, or source that is not approved for external processing. Review output is
judgment, not proof. Before logging or sharing, remove source items:

```sh
./examples/code-smell-review/review.sh path/to/file.go |
  jq 'del(.items)'
```

## Optional policy

The base review remains advisory. Callers can explicitly gate the high-confidence
cohesion answer, where the embedded question defines `high` as yes/safe:

```sh
review=$(./examples/code-smell-review/review.sh path/to/file.go)
printf '%s\n' "$review" |
  gev gate --as cohesion_policy \
    --value-pointer /_gev/code_smells/answers/cohesive/noul \
    --pass-min 0.80 --reject-max 0.30
```

`gate` is offline and does not make another API request. The gate is a caller
choice; the review itself does not block delivery.
