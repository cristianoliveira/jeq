# Probabilistic code-smell review

`review.sh` sends an explicitly selected, bounded set of source files through one
`jeq reduce` request. It adds probabilistic design evidence that complements
compiler errors, tests, linters, and static analysis; it is not a linter, proof,
a refactoring tool, or a replacement for deterministic checks.

The script reads only the paths passed as arguments, preserves their order, and
rejects missing, unreadable, non-regular, oversized, or more than 20 files. Each
file is limited to 256 KiB. The request embeds five independent positive Noul
questions. A successful review is advisory and exits 0.

## Run it

```sh
\
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

```bash
# Bash; requires jeq, jq, and TYPESAFE_API_KEY.
set -o pipefail
./examples/code-smell-review/review.sh examples/code-smell-review/fixtures/cohesive.go.txt |
jq 'del(.items)
  | ._jeq.code_smells.answers as $answers
  | ($answers | to_entries | map({id: .key, noul: .value.noul}) | sort_by(.noul)) as $dimensions
  | {dimensions: $dimensions, quality_floor: ($dimensions | map(.noul) | min)}'
```

## Optional policy

The base review remains advisory. Callers can explicitly gate the projected
quality floor with offline thresholds:

```bash
# Bash; requires jeq, jq, mktemp, and TYPESAFE_API_KEY.
set -o pipefail
review_file=$(mktemp) || exit $?
trap 'rm -f "$review_file"' EXIT
if ./examples/code-smell-review/review.sh examples/code-smell-review/fixtures/cohesive.go.txt >"$review_file"; then
  :
else
  review_status=$?
  exit "$review_status"
fi
policy_status=0
if jq 'del(.items)
  | ._jeq.code_smells.answers as $answers
  | ($answers | to_entries | map({id: .key, noul: .value.noul}) | sort_by(.noul)) as $dimensions
  | {dimensions: $dimensions, quality_floor: ($dimensions | map(.noul) | min)}' <"$review_file" |
  jeq gate --as code_smell_quality \
    --value-pointer /quality_floor \
    --pass-min 0.80 --reject-max 0.40; then
  policy_status=0
else
  policy_status=$?
fi
case "$policy_status" in
  0) printf '%s\n' 'policy passed' ;;
  10) printf '%s\n' 'policy rejected' >&2 ;;
  11) printf '%s\n' 'policy is uncertain' >&2 ;;
  *) exit "$policy_status" ;;
esac
exit "$policy_status"
```

`gate` is offline and does not make another API request. The gate is a caller
choice; the review itself does not block delivery. With these example thresholds,
values above or equal to `0.80` pass, values below or equal to `0.40` reject, and
values between them are uncertain (exit 11). These are illustrative policy
values, not calibrated proof that code is safe or defective.
