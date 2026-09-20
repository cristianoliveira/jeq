#!/usr/bin/env bash
# Compare every todo item with the same complete diff.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
JEQ_BIN=${JEQ_BIN:-jeq}
JEQ_MODEL=${JEQ_MODEL:-jev-latest}
PASS_MIN=${PASS_MIN:-0.85}
REJECT_MAX=${REJECT_MAX:-0.40}
TODO_FILE=${1:-}
DIFF_FILE=${2:-}

if [[ -z "$TODO_FILE" || -z "$DIFF_FILE" || ! -f "$TODO_FILE" || ! -r "$TODO_FILE" || ! -f "$DIFF_FILE" || ! -r "$DIFF_FILE" ]]; then
  printf 'usage: %s TODO.json DIFF.patch\n' "$0" >&2
  exit 2
fi

if ! jq -e 'type == "array" and length > 0 and all(.[]; (.id | type == "string") and (.description | type == "string"))' "$TODO_FILE" >/dev/null; then
  printf 'todo file must be a non-empty array of {id, description} objects\n' >&2
  exit 2
fi

if ! jq -e --argjson pass "$PASS_MIN" --argjson reject "$REJECT_MAX" '$pass > $reject and $pass <= 1 and $reject >= 0' <<< '{}' >/dev/null; then
  printf 'PASS_MIN must be greater than REJECT_MAX, with both values between 0 and 1\n' >&2
  exit 2
fi

status=0

while IFS= read -r todo; do
  state=$(jq -cn --argjson todo "$todo" --rawfile diff "$DIFF_FILE" '{todo:$todo,diff:$diff}')
  if response=$(printf '%s\n' "$state" | "$JEQ_BIN" ask \
      --questions "$SCRIPT_DIR/questions.json" \
      --state-json - \
      --model "$JEQ_MODEL"); then
    value=$(jq -er '.answers.fulfilled.noul | select(type == "number" and . >= 0 and . <= 1)' <<< "$response") || value=
  else
    value=
  fi

  if [[ -z "$value" ]]; then
    decision=uncertain
    status=11
    jq -cn --argjson todo "$todo" --argjson response "${response:-{}}" \
      '{todo:$todo,decision:"uncertain",reason:"missing_or_invalid_fulfillment_probability",response:$response}'
    continue
  fi

  decision=$(jq -rn --argjson value "$value" --argjson pass "$PASS_MIN" --argjson reject "$REJECT_MAX" \
    'if $value >= $pass then "fulfilled" elif $value <= $reject then "not_fulfilled" else "uncertain" end')
  if [[ "$decision" == uncertain ]]; then
    status=11
  elif [[ "$decision" == not_fulfilled && "$status" -eq 0 ]]; then
    status=10
  fi

  jq -cn --argjson todo "$todo" --argjson response "$response" \
    --arg decision "$decision" --argjson pass "$PASS_MIN" --argjson reject "$REJECT_MAX" \
    '{todo:$todo,decision:$decision,probability:($response.answers.fulfilled.noul),model:$response.model,confidence:($response.answers.fulfilled.confidence // null),thresholds:{pass_min:$pass,reject_max:$reject},usage:($response.usage // {}),answer:$response.answers.fulfilled}'
done < <(jq -c '.[]' "$TODO_FILE")

exit "$status"
