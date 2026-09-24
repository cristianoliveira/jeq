#!/usr/bin/env bash
# Read one diff from stdin and emit one deterministic policy receipt.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
JEQ_MODEL=${JEQ_MODEL:-jev-latest}
PASS_MIN=0.80
REVIEW_MAX=0.30
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-gate.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

JEQ_ARGS=(ask --questions "$SCRIPT_DIR/questions.json" --state-json-file - --model "$JEQ_MODEL")

run_jeq() {
  local state_json=$1
  local status
  if printf '%s\n' "$state_json" | jeq "${JEQ_ARGS[@]}" >"$TMP_DIR/out" 2>"$TMP_DIR/err"; then
    status=0
  else
    status=$?
  fi
  cat "$TMP_DIR/err" >&2
  cat "$TMP_DIR/out"
  return "$status"
}

diff_json=$(jq -Rs .)
if response=$(run_jeq "$diff_json"); then
  :
else
  status=$?
  printf '%s\n' "$response"
  exit "$status"
fi

usage_json=$(jq -c '.usage // {}' <<<"$response" 2>/dev/null || printf '{}')
if ! safe_to_ship=$(jq -er '
  .answers.safe_to_ship.noul as $value |
  if (($value | type) == "number" and $value >= 0 and $value <= 1) then $value else empty end
' <<<"$response" 2>/dev/null); then
  jq -cn --arg model "$JEQ_MODEL" --argjson usage "$usage_json" --argjson pass_min "$PASS_MIN" --argjson review_max "$REVIEW_MAX" '{workflow:"change-risk-gate",status:"uncertain",reason:"missing_or_invalid_safe_to_ship_signal",model:$model,usage:$usage,thresholds:{pass_min:$pass_min,review_or_block_max:$review_max}}'
  exit 11
fi

policy_status=$(jq -r --argjson safe "$safe_to_ship" --argjson pass_min "$PASS_MIN" --argjson review_max "$REVIEW_MAX" '
  if $safe >= $pass_min then "pass" elif $safe <= $review_max then "review_or_block" else "uncertain" end
' <<<"{}")
case "$policy_status" in
  pass) exit_code=0 ;;
  review_or_block) exit_code=10 ;;
  uncertain) exit_code=11 ;;
  *) exit_code=11 ;;
esac

jq -cn \
  --arg status "$policy_status" \
  --argjson safe "$safe_to_ship" \
  --arg model "$JEQ_MODEL" \
  --argjson usage "$usage_json" \
  --argjson pass_min "$PASS_MIN" \
  --argjson review_max "$REVIEW_MAX" \
  '{workflow:"change-risk-gate",status:$status,safe_to_ship:$safe,model:$model,usage:$usage,thresholds:{pass_min:$pass_min,review_or_block_max:$review_max}}'
exit "$exit_code"
