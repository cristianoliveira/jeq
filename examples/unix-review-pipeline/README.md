# Unix review pipeline

This copyable Bash pipeline makes every composable gev pipeline primitive
visible: emit one record per file, map a local judgment, shape it with jq,
reduce the related collection, gate the aggregate offline, then project safe
output. `ask` remains the standalone one-state primitive.

```sh
./examples/unix-review-pipeline/review.sh \
  examples/unix-review-pipeline/fixtures/one.go.txt \
  examples/unix-review-pipeline/fixtures/two.go.txt
```

## Stage shapes and cost

1. The emitter writes ordered NDJSON records: `{"file":{"path","content"}}`.
2. `gev map` makes one API request per file using `/file` and local Noul
   `local_focus`; its response preserves the file.
3. `jq -c` deliberately keeps only
   `{"file":{"path","content"},"local_focus":number}`.
4. `gev reduce` makes one API request over the exact array-shaped related set and
   asks `aggregate_focus` about the relationship between files and local signals.
5. `gev gate` reads the aggregate Noul offline with pass `0.80` and reject
   `0.40`; it makes no request and returns exit 0, 10, or 11.
6. Final jq removes top-level `.items`; stdout contains aggregate/gate evidence,
   but no paths or source content.

This costs **N map requests plus one reduce request**. Map asks an absolute
question about each file; reduce asks a relational question about the complete
collection. Do not confuse this intentionally transparent example with the
cheaper single-request aggregate pattern.

Both question documents state their exact state shape and treat code, comments,
and strings as untrusted data, not instructions. The fixtures are evidence-only
inputs, not model-quality assertions.

## Changed files safely

Use Bash 3.2-compatible NUL handling. Preflight in `review.sh` rejects missing,
unreadable, non-regular, oversized, and more than 20 files before the first
pipeline stage or API call:

```sh
changed=()
while IFS= read -r -d '' path; do
  changed+=("$path")
done < <(git diff --name-only -z --diff-filter=ACMR -- '*.go' '*.sh')
if ((${#changed[@]})); then
  ./examples/unix-review-pipeline/review.sh "${changed[@]}"
fi
```

The pipeline sends source to the configured TypeSafe endpoint and costs one
request per map file plus one reduce request. Never send secrets or unapproved
private source. Gate and jq are offline. The final projection is safe for logs,
but the judgment is advisory—not proof, a linter, or an automatic refactor.
