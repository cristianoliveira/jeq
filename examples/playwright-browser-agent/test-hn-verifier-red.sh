#!/usr/bin/env bash
# Red acceptance test for TASK-0064 verification. It intentionally fails until
# hn-browser.sh verifies the selected discussion after Jev chooses DONE.
set -euo pipefail

root=$(cd "$(dirname "$0")" && pwd)
bin=$(mktemp -d)
capture="$bin/requests"
mkdir "$capture"
trap 'rm -rf "$bin"' EXIT

cat >"$bin/playwright-cli" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
state_file=${TMP_STATE:?}
state=$(cat "$state_file" 2>/dev/null || echo 0)
case "$*" in
  *open*) printf 'open %s\n' "$*" >>"$EVENTS" ;;
  *snapshot*)
    printf 'snapshot state=%s\n' "$state" >>"$EVENTS"
    case "$state" in
      0) printf '%s\n' '- link "past" [ref=e1]:' '  - /url: "front"' ;;
      1) printf '%s\n' '- link "yesterday" [ref=e2]:' '  - /url: "front?day=2026-01-01"' ;;
      2)
        printf '%s\n' \
          '- link "120 comments" [ref=e3]:' '  - /url: "item?id=10"' \
          '- link "525 comments" [ref=e4]:' '  - /url: "item?id=42"' \
          '- link "525 comments" [ref=e5]:' '  - /url: "item?id=43"'
        ;;
      3)
        printf '%s\n' \
          '- heading "Chosen story | Hacker News" [ref=e6]' \
          '- text: "Stories from January 1, 2026 (UTC)"' \
          '- text: "alice: first bounded top-level comment"' \
          '- text: "bob: second bounded top-level comment"'
        ;;
    esac
    ;;
  *click*)
    printf 'click %s state=%s\n' "$*" "$state" >>"$EVENTS"
    printf '%s\n' "$((state + 1))" >"$state_file"
    ;;
  *close*) printf 'close\n' >>"$EVENTS"; touch "$TMP_CLOSED" ;;
  *) printf 'other %s\n' "$*" >>"$EVENTS" ;;
esac
SH

cat >"$bin/jeq" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
n=$(find "$CAPTURE" -type f -name 'request-*.json' | wc -l | tr -d ' ')
input="$CAPTURE/request-$n.json"
cat >"$input"
case "$n" in
  0) choice=c1;;
  1) choice=c1;;
  2) choice=c2;;
  3) choice=DONE;;
  *) choice=BLOCKED;;
esac
printf '%s\n' "$choice" >>"$DECISIONS"
printf '{"answers":{"choice":{"choice":"%s"}}}\n' "$choice"
SH

chmod +x "$bin"/*
printf '0\n' >"$bin/state"
: >"$bin/events"
: >"$bin/decisions"
export PATH="$bin:$PATH"
export TMP_STATE="$bin/state" TMP_CLOSED="$bin/closed" EVENTS="$bin/events"
export CAPTURE="$capture" DECISIONS="$bin/decisions"

set +e
"$root/hn-browser.sh" >"$bin/stdout" 2>"$bin/stderr"
status=$?
set -e

count=$(find "$capture" -type f -name 'request-*.json' | wc -l | tr -d ' ')
if [[ "$count" -ne 4 ]]; then
  echo "FAIL: first unmet verifier criterion: expected 4 jeq requests, got $count" >&2
  cat "$bin/stderr" >&2
  exit 1
fi

expected_decisions=$'c1\nc1\nc2\nDONE'
if [[ "$(cat "$bin/decisions")" != "$expected_decisions" ]]; then
  echo "FAIL: Jev decisions changed" >&2
  cat "$bin/decisions" >&2
  exit 1
fi

if ! jq -e '.questions.choice.criteria.c1 == "120 comments" and .questions.choice.criteria.c2 == "525 comments" and .questions.choice.criteria.c3 == "525 comments"' "$capture/request-2.json" >/dev/null; then
  echo "FAIL: tie-page candidates were not exposed as bounded Choice options" >&2
  exit 1
fi

for request in "$capture"/request-*.json; do
  if ! jq -e '(.state | has("expected_item_id") or has("expected_max_comments") or has("verification") or has("top_level_comments") or has("selected_item_id")) | not' "$request" >/dev/null; then
    echo "FAIL: verifier state leaked into Jev request: $request" >&2
    exit 1
  fi
done

if [[ ! -e "$TMP_CLOSED" ]]; then
  echo "FAIL: browser cleanup did not run" >&2
  exit 1
fi

if [[ -z "$(tr -d '[:space:]' <"$bin/stdout")" ]]; then
  echo "FAIL: first production verifier failure: no post-DONE verification result (status $status)" >&2
  cat "$bin/stderr" >&2
  exit 1
fi

if ! jq -e '
  .verification.previous_utc_date == "2026-01-01" and
  (.verification.selected_item_id == "42" or .verification.selected_item_id == "43") and
  .verification.selected_comment_count == 525 and
  .verification.maximum_comment_count == 525 and
  .verification.selected_is_maximum == true and
  (.verification.top_level_comments | length <= 5) and
  (.verification.top_level_comments | all(.text | length <= 600))
' "$bin/stdout" >/dev/null; then
  echo "FAIL: post-DONE verification did not prove the date, maximum tie, and bounded comments" >&2
  cat "$bin/stdout" >&2
  exit 1
fi

printf 'PASS verifier acceptance\n'
