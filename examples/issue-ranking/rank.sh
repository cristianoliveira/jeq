#!/usr/bin/env bash
# Read issue NDJSON, evaluate each line once, and emit correlated NDJSON.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
JEQ_BIN=${JEQ_BIN:-jeq}
JEQ_MODEL=${JEQ_MODEL:-jev-latest}
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-rank.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

JEQ_ARGS=(ask --questions "$SCRIPT_DIR/questions.json" --state-json - --model "$JEQ_MODEL")

run_jeq() {
  local state_json=$1
  local status
  if printf '%s\n' "$state_json" | "$JEQ_BIN" "${JEQ_ARGS[@]}" >"$TMP_DIR/out" 2>"$TMP_DIR/err"; then
    status=0
  else
    status=$?
  fi
  cat "$TMP_DIR/err" >&2
  cat "$TMP_DIR/out"
  return "$status"
}

order=0
while IFS= read -r line || [[ -n "$line" ]]; do
  order=$((order + 1))
  if ! id=$(printf '%s\n' "$line" | jq -er 'if (.id | type) == "string" and (.id | length) > 0 then .id else empty end' 2>/dev/null); then
    jq -cn --argjson order "$order" '{workflow:"issue-ranking",status:"input_invalid",order:$order,reason:"each record requires a non-empty string id"}'
    exit 2
  fi
  if ! state_json=$(printf '%s\n' "$line" | jq -ce . 2>/dev/null); then
    jq -cn --arg id "$id" --argjson order "$order" '{workflow:"issue-ranking",status:"input_invalid",id:$id,order:$order,reason:"record is not valid JSON"}'
    exit 2
  fi

  if response=$(run_jeq "$state_json"); then
    :
  else
    status=$?
    printf '%s\n' "$response"
    exit "$status"
  fi

  if ! receipt=$(jq -ce --arg id "$id" --argjson order "$order" '
    .answers.priority.score as $priority |
    .answers.impact.score as $impact |
    .answers.priority.confidence as $priority_confidence |
    .answers.impact.confidence as $impact_confidence |
    if (($priority | type) != "number" or ($impact | type) != "number" or
        ($priority_confidence | type) != "number" or ($impact_confidence | type) != "number" or
        $priority_confidence < 0 or $priority_confidence > 1 or
        $impact_confidence < 0 or $impact_confidence > 1) then
      error("missing or invalid score evidence")
    else
      {
        workflow: "issue-ranking",
        id: $id,
        order: $order,
        priority_score: $priority,
        impact_score: $impact,
        priority_confidence: $priority_confidence,
        impact_confidence: $impact_confidence,
        rank_score: (($priority * 10) + $impact),
        model: .model,
        usage: (.usage // {})
      }
    end
  ' <<<"$response" 2>/dev/null); then
    jq -cn --arg id "$id" --argjson order "$order" '{workflow:"issue-ranking",status:"uncertain",id:$id,order:$order,reason:"response did not contain numeric scores and confidence evidence"}'
    exit 11
  fi
  printf '%s\n' "$receipt"
done
