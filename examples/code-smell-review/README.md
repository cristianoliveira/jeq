# Probabilistic code-smell review

`review.sh` sends an explicitly selected, bounded set of source files through one
`gev reduce` request. It adds probabilistic design evidence that complements
compiler errors, tests, linters, and static analysis; it is not a linter, proof,
a refactoring tool, or a replacement for deterministic checks.

The script reads only the paths passed as arguments, preserves their order, and
rejects missing, unreadable, non-regular, oversized, or more than 20 files. Each
file is limited to 256 KiB. The request embeds five independent positive Noul
questions. A successful review is advisory and exits 0.

## Run it

```sh
GEV_BIN=gev JQ_BIN=jq \
  ./examples/code-smell-review/review.sh \
  internal/cli/map.go internal/domain/pipeline/reduce.go
```

The labeled fixtures are input evidence for demonstrations only, not compiled
examples or deterministic model tests:

```sh
./examples/code-smell-review/review.sh \
  examples/code-smell-review/fixtures/cohesive.go.txt
./examples/code-smell-review/review.sh \
  examples/code-smell-review/fixtures/duplicated-policy.go.txt
```

For changed files, use NUL-delimited Git output so spaces and newlines in paths
are not split by the shell (this loop also works with Bash 3.2):

```sh
changed=()
while IFS= read -r -d '' path; do
  changed+=("$path")
done < <(
  git diff --name-only -z --diff-filter=ACMR -- '*.go' '*.sh'
)
if ((${#changed[@]})); then
  ./examples/code-smell-review/review.sh "${changed[@]}"
fi
```

The review sends source code to the configured TypeSafe endpoint and spends one
API request per invocation. Do not send credentials, secrets, generated private
data, or source that is not approved for external processing. Review output is
judgment, not proof.

Each Noul is the probability of its own literal yes-condition, not an intensity
score or confidence value. Several dimensions may be low together because
concerns can co-occur. Treat low dimensions as review signals, not proof; review
one related concern at a time. Exclude generated, vendor, and irrelevant files
to avoid diluting context with unrelated code.

Before logging or sharing, remove source items and render only the dimensions and
floor. The projection sorts dimensions from lowest to highest and computes the
top-level `quality_floor` as their minimum:

```sh
./examples/code-smell-review/review.sh path/to/file.go |
jq 'del(._gev.code_smells.items)
  | ._gev.code_smells.answers as $answers
  | ($answers | to_entries | map({id: .key, noul: .value.noul}) | sort_by(.noul)) as $dimensions
  | {dimensions: $dimensions, quality_floor: ($dimensions | map(.noul) | min)}'
```

## Optional policy

The base review remains advisory. Callers can explicitly gate the projected
quality floor with offline thresholds:

```sh
review=$(./examples/code-smell-review/review.sh path/to/file.go)
printf '%s\n' "$review" |
jq 'del(._gev.code_smells.items)
  | ._gev.code_smells.answers as $answers
  | ($answers | to_entries | map({id: .key, noul: .value.noul}) | sort_by(.noul)) as $dimensions
  | {dimensions: $dimensions, quality_floor: ($dimensions | map(.noul) | min)}' |
  gev gate --as code_smell_quality \
    --value-pointer /quality_floor \
    --pass-min 0.80 --reject-max 0.40
```

`gate` is offline and does not make another API request. The gate is a caller
choice; the review itself does not block delivery.
