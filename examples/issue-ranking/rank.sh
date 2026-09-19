#!/usr/bin/env bash
# Read issue NDJSON, evaluate each line once, and emit correlated NDJSON.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
GEV_BIN=${GEV_BIN:-gev}
GEV_MODEL=${GEV_MODEL:-jev-latest}
GEV_BASE_URL=${GEV_BASE_URL:-}
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/gev-rank.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

GEV_ARGS=(ask --questions "$SCRIPT_DIR/questions.json" --state-json - --model "$GEV_MODEL")
if [[ -n "$GEV_BASE_URL" ]]; then
  GEV_ARGS+=(--base-url "$GEV_BASE_URL")
fi

run_gev() {
  local state_json=$1
  local status
  if printf '%s\n' "$state_json" | "$GEV_BIN" "${GEV_ARGS[@]}" >"$TMP_DIR/out" 2>"$TMP_DIR/err"; then
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

  if response=$(run_gev "$state_json"); then
    :
  else
    status=$?
    printf '%s\n' "$response"
    exit "$status"
  fi

  if ! receipt=$(jq -ce --arg id "$id" --argjson order "$order" '
    .answers.priority.score as $priority |
    .answers.impact.score as $impact |
    if (($priority | type) != "number" or ($impact | type) != "number") then
      error("missing score")
    else
      {
        workflow: "issue-ranking",
        id: $id,
        order: $order,
        priority_score: $priority,
        impact_score: $impact,
        rank_score: (($priority * 10) + $impact),
        model: .model
      }
    end
  ' <<<"$response" 2>/dev/null); then
    jq -cn --arg id "$id" --argjson order "$order" '{workflow:"issue-ranking",status:"uncertain",id:$id,order:$order,reason:"response did not contain numeric priority and impact scores"}'
    exit 11
  fi
  printf '%s\n' "$receipt"
done
