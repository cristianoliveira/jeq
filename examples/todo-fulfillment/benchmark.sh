#!/usr/bin/env bash
# Run labeled todo/diff cases and report simple accuracy evidence.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
JEQ_REVIEW=${JEQ_REVIEW:-$SCRIPT_DIR/review.sh}
CASES_DIR=${1:-$SCRIPT_DIR/cases}
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-todo-benchmark.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ ! -d "$CASES_DIR" ]]; then
  printf 'cases directory not found: %s\n' "$CASES_DIR" >&2
  exit 2
fi

results=$TMP_DIR/results.ndjson
: > "$results"
for case_dir in "$CASES_DIR"/*/; do
  [[ -d "$case_dir" ]] || continue
  todo_file=$case_dir/todos.json
  diff_file=$case_dir/change.patch
  [[ -f "$todo_file" && -f "$diff_file" ]] || {
    printf 'case must contain todos.json and change.patch: %s\n' "$case_dir" >&2
    exit 2
  }

  set +e
  "$JEQ_REVIEW" "$todo_file" "$diff_file" > "$TMP_DIR/case.ndjson"
  case_status=$?
  set -e
  jq -c --arg case "$(basename "$case_dir")" --argjson case_status "$case_status" \
    '. + {case:$case,case_status:$case_status}' "$TMP_DIR/case.ndjson" >> "$results"
done

jq -s '
  map({case, id:.todo.id, expected:(.todo.expected // null), observed:
    (if .decision == "fulfilled" then "fulfilled" elif .decision == "not_fulfilled" then "not_fulfilled" else "uncertain" end),
    probability:(.probability // null)}) as $items |
  ($items | map(select(.expected != null))) as $labeled |
  {
    items: $items,
    labeled_count: ($labeled | length),
    correct: ($labeled | map(select(.expected == .observed and .observed != "uncertain")) | length),
    accuracy: (if ($labeled | length) == 0 then null else (($labeled | map(select(.expected == .observed and .observed != "uncertain")) | length) / ($labeled | length)) end),
    uncertain_count: ($items | map(select(.observed == "uncertain")) | length),
    false_positives: ($labeled | map(select(.expected == "not_fulfilled" and .observed == "fulfilled")) | length),
    false_negatives: ($labeled | map(select(.expected == "fulfilled" and .observed == "not_fulfilled")) | length)
  }
' "$results"
