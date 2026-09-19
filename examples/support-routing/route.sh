#!/usr/bin/env bash
# Read one support ticket from stdin and emit one safe routing receipt.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
GEV_BIN=${GEV_BIN:-gev}
GEV_MODEL=${GEV_MODEL:-jev-latest}
GEV_BASE_URL=${GEV_BASE_URL:-}
ROUTE_CONFIDENCE_MIN=0.70
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/gev-route.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

# Pass the optional endpoint as an argument. Never build a shell command from
# model output; all values below are data passed as quoted arguments.
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

ticket_json=$(jq -Rs .)
if response=$(run_gev "$ticket_json"); then
  :
else
  status=$?
  printf '%s\n' "$response"
  exit "$status"
fi

jq -c --arg model "$GEV_MODEL" --argjson min_confidence "$ROUTE_CONFIDENCE_MIN" '
  .answers.route as $route |
  ($route.choice // null) as $choice |
  (if ($route.confidence | type) == "number" then $route.confidence else 0 end) as $confidence |
  (if (.answers.urgent.noul | type) == "number" then .answers.urgent.noul else null end) as $urgent |
  (if (.answers.escalate.noul | type) == "number" then .answers.escalate.noul else null end) as $escalate |
  (if ($confidence < $min_confidence) then "human_review"
   elif $choice == "billing" then "billing_queue"
   elif $choice == "technical" then "technical_queue"
   elif $choice == "sales" then "sales_queue"
   else "human_review"
   end) as $action |
  {
    workflow: "support-routing",
    action: $action,
    route: $choice,
    route_confidence: $confidence,
    urgent: $urgent,
    escalate: $escalate,
    model: (.model // $model),
    usage: (.usage // {}),
    policy: {minimum_route_confidence: $min_confidence, unknown_route: "human_review"}
  }
' <<<"$response"
