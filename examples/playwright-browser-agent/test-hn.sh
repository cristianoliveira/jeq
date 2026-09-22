#!/usr/bin/env bash
# Red acceptance test for TASK-0064 verification. It intentionally fails until
# hn-browser.sh verifies fresh post-DONE browser evidence.
set -euo pipefail

root=$(cd "$(dirname "$0")" && pwd)
bin=$(mktemp -d)
trap 'rm -rf "$bin"' EXIT

if date -u -v-1d +%F >/dev/null 2>&1; then
  yesterday=$(date -u -v-1d +%F)
else
  yesterday=$(date -u -d 'yesterday' +%F)
fi

today=$(date -u +%F)
if date -u -v-2d +%F >/dev/null 2>&1; then
  wrong_day=$(date -u -v-2d +%F)
else
  wrong_day=$(date -u -d '2 days ago' +%F)
fi

cat >"$bin/playwright-cli" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
state=$(cat "$TMP_STATE" 2>/dev/null || echo 0)
case "$*" in
  *open*)
    printf 'open %s\n' "$*" >>"$EVENTS"
    if [[ "$SCENARIO" == open-hang ]]; then sleep 2; fi
    ;;
  *snapshot*)
    if [[ "$SCENARIO" == snapshot-hang ]]; then sleep 2; fi
    printf 'snapshot state=%s\n' "$state" >>"$EVENTS"
    case "$state" in
      0) printf '%s\n' '- link "past" [ref=e1]:' '  - /url: "front"' ;;
      1)
        date_url="$YESTERDAY"
        [[ "$SCENARIO" == wrongday ]] && date_url="$WRONG_DAY"
        printf '%s\n' "- link \"yesterday $YESTERDAY\" [ref=e2]:" "  - /url: \"front?day=$date_url\""
        if [[ "$SCENARIO" == oversized-snapshot ]]; then
          head -c 70000 < /dev/zero | tr '\0' x
          printf '\n'
        fi
        ;;
      2)
        printf '%s\n' \
          '- link "120 comments" [ref=e3]:' '  - /url: "item?id=10"' \
          '- link "525 comments" [ref=e4]:' '  - /url: "item?id=42"'
        if [[ "$SCENARIO" == tie ]]; then
          printf '%s\n' '- link "525 comments" [ref=e5]:' '  - /url: "item?id=43"'
        else
          printf '%s\n' '- link "500 comments" [ref=e5]:' '  - /url: "item?id=43"'
        fi
        if [[ "$SCENARIO" == candidate-count ]]; then
          for i in $(seq 6 70); do
            printf '%s\n' "- link \"100 comments\" [ref=e$i]:" "  - /url: \"item?id=$((1000 + i))\""
          done
        elif [[ "$SCENARIO" == long-label ]]; then
          printf '%s\n' "- link \"$(printf 'x%.0s' $(seq 1 280)) comments\" [ref=e6]:" '  - /url: "item?id=1006"'
        elif [[ "$SCENARIO" == unsafe-links ]]; then
          printf '%s\n' \
            '- link "600 comments" [ref=e6]:' '  - /url: "https://news.ycombinator.com:8443/item?id=55"' \
            '- link "601 comments" [ref=e7]:' '  - /url: "item?id=56&vote=1"' \
            '- link "602 comments" [ref=e8]:' '  - /url: "item?id=57&submit=1"' \
            '- link "603 comments" [ref=e10]:' '  - /url: "item?id=58&reply=1"' \
            '- link "delete action" [ref=e11]:' '  - /url: "delete?id=59"' \
            '- link "upvote action" [ref=e12]:' '  - /url: "upvote?id=60"' \
            '- link "query delete action" [ref=e13]:' '  - /url: "item?id=61&delete=1"'
        else
          printf '%s\n' '- link "999 comments" [ref=e9]:' "  - /url: \"front?day=$YESTERDAY\""
        fi
        ;;
      3)
        printf '%s\n' \
          '- heading "What happened ... | Hacker News" [ref=e6]' \
          "- text: \"Stories from $YESTERDAY (UTC)\"" \
          '- text: "alice: first bounded top-level comment"' \
          '- text: "bob: second bounded top-level comment"'
        ;;
    esac
    ;;
  *eval*)
    if [[ "$SCENARIO" == eval-hang ]]; then sleep 2; fi
    # Fixed, code-owned results for independent inspection.
    case "$*" in
      *location.href*)
        printf 'eval location.href state=%s\n' "$state" >>"$EVENTS"
        case "$state" in
          0) location='https://news.ycombinator.com/' ;;
          1) if [[ "$SCENARIO" == wrongday ]]; then location="https://news.ycombinator.com/front?day=$WRONG_DAY"; else location="https://news.ycombinator.com/front?day=$YESTERDAY"; fi ;;
          2) if [[ "$SCENARIO" == redirect ]]; then location='https://news.ycombinator.com/item?id=999'; else location="https://news.ycombinator.com/front?day=$YESTERDAY"; fi ;;
          *) case "$SCENARIO" in
               nonmax) location='https://news.ycombinator.com/item?id=10' ;;
               redirect) location='https://news.ycombinator.com/item?id=43' ;;
               *) location='https://news.ycombinator.com/item?id=42' ;;
             esac ;;
        esac
        printf '%s\n' '### Result' "\"$location\""
        ;;
      *document.title*)
        printf 'eval document.title state=%s\n' "$state" >>"$EVENTS"
        if [[ "$SCENARIO" == badtitle && "$state" == 3 ]]; then
          printf '%s\n' '### Result' '"Error page"'
        else
          case "$state" in
            0) printf '%s\n' '### Result' '"Hacker News"' ;;
            1) printf '%s\n' '### Result' '"Hacker News: past"' ;;
            2) printf '%s\n' '### Result' '"Hacker News: yesterday"' ;;
            *) printf '%s\n' '### Result' '"What happened ... | Hacker News"' ;;
          esac
        fi
        ;;
      *document.body.innerText*)
        [[ "$SCENARIO" == eval-hang ]] && sleep 5
        if [[ "$*" == *'slice(0,12000)'* ]]; then printf 'eval body bounded state=%s\n' "$state" >>"$EVENTS"; else printf 'eval body unbounded state=%s\n' "$state" >>"$EVENTS"; fi
        if [[ "$SCENARIO" == oversized-eval ]]; then
          printf '%s\n' '### Result' '"'; head -c 70000 < /dev/zero | tr '\0' x; printf '%s\n' '"'
        elif [[ "$SCENARIO" == badbody ]]; then printf '%s\n' '### Result' '"An unrelated error page."'; else printf '%s\n' '### Result' '"An ordinary discussion about what happened."'; fi
        ;;
      *tr.comtr*)
        [[ "$SCENARIO" == eval-hang ]] && sleep 5
        if [[ "$*" == *'JSON.stringify(Array.from(document.querySelectorAll("tr.comtr")).filter(function(row){var indent=row.querySelector("td.ind img");return indent && Number(indent.getAttribute("width"))===0;}).slice(0,5).map(function(row)'* && "$*" == *'commtext'* && "$*" == *'hnuser'* && "$*" == *'slice(0,600)'* ]]; then printf 'eval comments structural state=%s\n' "$state" >>"$EVENTS"; else printf 'eval comments invalid state=%s\n' "$state" >>"$EVENTS"; fi
        case "$SCENARIO" in
          malformed-comments) printf '%s\n' '### Result' 'not-json' ;;
          missing-depth-comments) printf '%s\n' '### Result' '[{"user":"alice","text":"missing depth"}]' ;;
          empty-comments) printf '%s\n' '### Result' '[]' ;;
          *) printf '%s\n' '### Result' '[{"user":"alice","text":"first bounded top-level comment","depth":0},{"user":"bob","text":"second bounded top-level comment","depth":0}]' ;;
        esac
        ;;
      *) printf 'eval unknown state=%s\n' "$state" >>"$EVENTS"; printf '%s\n' '### Result' '{}' ;;
    esac
    ;;
  *click*)
    printf 'click %s state=%s\n' "$*" "$state" >>"$EVENTS"
    if [[ "$SCENARIO" == click-hang ]]; then sleep 2; fi
    printf '%s\n' "$((state + 1))" >"$TMP_STATE"
    ;;
  *close*)
    printf 'close-start\n' >>"$EVENTS"
    if [[ "$SCENARIO" == close-hang ]]; then sleep 2; fi
    printf 'close\n' >>"$EVENTS"; touch "$TMP_CLOSED"
    ;;
  *) printf 'other %s\n' "$*" >>"$EVENTS" ;;
esac
SH

cat >"$bin/jeq" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
n=$(find "$CAPTURE" -type f -name 'request-*.json' | wc -l | tr -d ' ')
input="$CAPTURE/request-$n.json"
cat >"$input"
case "$SCENARIO:$n" in
  jeq-hang:*) sleep 2 ;;
  malformed-response:*) printf 'not-json\n'; exit 0 ;;
  oversized-response:*) printf '{"padding":"'; head -c 70000 < /dev/zero | tr '\0' x; printf '%s\n' '","answers":{"choice":{"choice":"c1"}}}'; exit 0 ;;
  stale-choice:0) choice=c99;;
  unknown-choice:0) choice=not-a-choice;;
  blocked-choice:0) choice=BLOCKED;;
  tie:0|nonmax:0|redirect:0|wrongday:0|badtitle:0|badbody:0|empty-comments:0|malformed-comments:0|missing-depth-comments:0|candidate-count:0|long-label:0|unsafe-links:0|headed:0|trace:0|oversized-eval:0|oversized-snapshot:0|click-hang:0|close-hang:0) choice=c1;;
  tie:1|nonmax:1|redirect:1|wrongday:1|badtitle:1|badbody:1|empty-comments:1|malformed-comments:1|missing-depth-comments:1|candidate-count:1|long-label:1|unsafe-links:1|headed:1|trace:1|oversized-eval:1|oversized-snapshot:1|click-hang:1|close-hang:1) choice=c1;;
  tie:2|redirect:2|wrongday:2|badtitle:2|badbody:2|empty-comments:2|malformed-comments:2|missing-depth-comments:2|candidate-count:2|long-label:2|unsafe-links:2|headed:2|trace:2|oversized-eval:2|oversized-snapshot:2|click-hang:2|close-hang:2) choice=c2;;
  nonmax:2) choice=c1;;
  tie:3|nonmax:3|redirect:3|wrongday:3|badtitle:3|badbody:3|empty-comments:3|malformed-comments:3|missing-depth-comments:3|candidate-count:3|long-label:3|unsafe-links:3|headed:3|trace:3|oversized-eval:3|oversized-snapshot:3|click-hang:3|close-hang:3)
    choice=DONE
    printf 'decision DONE\n' >>"$EVENTS"
    ;;
  *) choice=BLOCKED;;
esac
printf '%s\n' "$choice" >>"$DECISIONS"
printf '{"model":"jev-latest","usage":{"input_tokens":11,"output_tokens":2},"answers":{"choice":{"choice":"%s"}}}\n' "$choice"
SH

chmod +x "$bin"/*

run_case() {
  local scenario=$1
  local case_dir="$bin/$scenario"
  mkdir -p "$case_dir/requests"
  printf '0\n' >"$case_dir/state"
  : >"$case_dir/events"
  : >"$case_dir/decisions"
  export PATH="$bin:$PATH"
  export SCENARIO="$scenario" YESTERDAY="$yesterday" WRONG_DAY="$wrong_day" TODAY="$today" JEQ_SUBPROCESS_TIMEOUT_SECONDS=1
  unset JEQ_GOAL
  if [[ "$scenario" == oversized-request ]]; then JEQ_GOAL=$(printf 'g%.0s' $(seq 1 100000)); export JEQ_GOAL; fi
  export TMP_STATE="$case_dir/state" TMP_CLOSED="$case_dir/closed" EVENTS="$case_dir/events"
  export CAPTURE="$case_dir/requests" DECISIONS="$case_dir/decisions"

  local started finished
  started=$(date +%s)
  local -a command=("$root/hn-browser.sh")
  [[ "$scenario" == headed ]] && command+=(--headed)
  export JEQ_BROWSER_TRACE=0
  [[ "$scenario" == trace ]] && export JEQ_BROWSER_TRACE=1
  set +e
  timeout --foreground 3s "${command[@]}" >"$case_dir/stdout" 2>"$case_dir/stderr"
  local status=$?
  set -e
  finished=$(date +%s)
  local elapsed=$((finished - started))

  local count
  count=$(find "$case_dir/requests" -type f -name 'request-*.json' | wc -l | tr -d ' ')
  if [[ "$scenario" == open-hang || "$scenario" == snapshot-hang || "$scenario" == eval-hang || "$scenario" == click-hang || "$scenario" == close-hang || "$scenario" == jeq-hang ]]; then
    local hang_failed=0
    if [[ "$scenario" == close-hang ]]; then
      [[ "$elapsed" -gt 5 ]] && hang_failed=1
    else
      { [[ "$status" -eq 0 ]] || [[ "$elapsed" -gt 3 ]]; } && hang_failed=1
    fi
    ! grep -q '^close-start$' "$case_dir/events" && hang_failed=1
    if [[ "$hang_failed" -ne 0 ]]; then
      echo "FAIL [$scenario]: subprocess was not bounded and cleaned up (status=$status elapsed=${elapsed}s)" >&2
      cat "$case_dir/events" >&2
      return 1
    fi
    return 0
  fi
  if [[ "$scenario" == oversized-snapshot || "$scenario" == malformed-response || "$scenario" == oversized-response || "$scenario" == oversized-request ]]; then
    local expected_count=1
    [[ "$scenario" == oversized-request ]] && expected_count=0
    if [[ "$status" -eq 0 || "$count" -ne "$expected_count" ]]; then
      echo "FAIL [$scenario]: oversized/malformed input was not rejected before the next Jev step (status=$status requests=$count expected=$expected_count)" >&2
      return 1
    fi
    [[ -e "$TMP_CLOSED" ]] || { echo "FAIL [$scenario]: cleanup did not run" >&2; return 1; }
    return 0
  fi
  if [[ "$scenario" == headed ]] && ! grep -q -- '--headed' "$case_dir/events"; then
    echo "FAIL [headed]: paid invocation did not pass --headed to Playwright open" >&2
    return 1
  fi
  if [[ "$scenario" == stale-choice || "$scenario" == unknown-choice || "$scenario" == blocked-choice ]]; then
    if [[ "$status" -eq 0 || "$count" -ne 1 ]] || grep -q '^click ' "$case_dir/events" || [[ ! -e "$TMP_CLOSED" ]]; then
      echo "FAIL [$scenario]: unsafe Jev choice was not rejected before click with cleanup" >&2
      cat "$case_dir/events" >&2
      return 1
    fi
    return 0
  fi
  if [[ "$scenario" == wrongday || "$scenario" == redirect || "$scenario" == badtitle ]]; then
    local invalid_state=1
    [[ "$scenario" == redirect ]] && invalid_state=2
    [[ "$scenario" == badtitle ]] && invalid_state=3
    if [[ "$status" -eq 0 || "$count" -ne "$invalid_state" ]]; then
      echo "FAIL [$scenario]: Jev request continued after invalid observed state (status=$status requests=$count expected=$invalid_state)" >&2
      cat "$case_dir/events" >&2
      return 1
    fi
    if ! grep -q "^eval location.href state=$invalid_state$" "$case_dir/events" || ! grep -q "^eval document.title state=$invalid_state$" "$case_dir/events"; then
      echo "FAIL [$scenario]: failure did not independently inspect the invalid observed page" >&2
      cat "$case_dir/events" >&2
      return 1
    fi
    if [[ ! -e "$TMP_CLOSED" ]]; then
      echo "FAIL [$scenario]: browser cleanup did not run" >&2
      return 1
    fi
    return 0
  fi
  if [[ "$count" -ne 4 ]]; then
    echo "FAIL [$scenario]: expected 4 jeq requests, got $count" >&2
    cat "$case_dir/stderr" >&2
    return 1
  fi

  for observed_state in 0 1 2; do
    if ! grep -q "^eval location.href state=$observed_state$" "$case_dir/events" || ! grep -q "^eval document.title state=$observed_state$" "$case_dir/events"; then
      echo "FAIL [$scenario]: location.href and document.title were not evaluated for state $observed_state" >&2
      cat "$case_dir/events" >&2
      return 1
    fi
  done

  local expected_url expected_title request_number
  for request in "$case_dir"/requests/request-*.json; do
    request_number=${request##*-}; request_number=${request_number%.json}
    case "$request_number" in
      0) expected_url='https://news.ycombinator.com/'; expected_title='Hacker News' ;;
      1|2) expected_url="https://news.ycombinator.com/front?day=$yesterday"; [[ "$request_number" == 1 ]] && expected_title='Hacker News: past' || expected_title='Hacker News: yesterday' ;;
      3)
        if [[ "$scenario" == nonmax ]]; then expected_url='https://news.ycombinator.com/item?id=10'; else expected_url='https://news.ycombinator.com/item?id=42'; fi
        expected_title='What happened ... | Hacker News'
        ;;
      *) echo "FAIL [$scenario]: unexpected request number $request_number" >&2; return 1 ;;
    esac
    if ! jq -e --arg url "$expected_url" --arg title "$expected_title" '.state.current_url == $url and .state.title == $title' "$request" >/dev/null; then
      echo "FAIL [$scenario]: request $request_number did not use observed URL/title" >&2
      jq '.state | {current_url,title}' "$request" >&2
      return 1
    fi
  done

  if [[ "$scenario" == candidate-count || "$scenario" == long-label ]]; then
    if ! jq -e '(.state.visible_text | length) <= 12000 and ([.questions.choice.criteria | to_entries[] | select(.key | test("^c[0-9]+$"))] | length) <= 64 and all(.questions.choice.criteria[]; length <= 256)' "$case_dir/requests/request-2.json" >/dev/null || [[ $(wc -c <"$case_dir/requests/request-2.json") -gt 65536 ]]; then
      echo "FAIL [$scenario]: candidate, label, visible-text, or request byte cap was exceeded" >&2
      return 1
    fi
    if [[ "$scenario" == long-label ]] && jq -e '.questions.choice.criteria | has("c4")' "$case_dir/requests/request-2.json" >/dev/null; then
      echo "FAIL [$scenario]: oversized candidate was not filtered" >&2
      return 1
    fi
    if ! grep -Fq '.[-8:]' "$root/hn-browser.sh" || ! awk '/^decision DONE$/{exit} /^snapshot state=/{n++} END{exit n > 4}' "$case_dir/events"; then
      echo "FAIL [$scenario]: recent-decision or step cap is missing" >&2
      return 1
    fi
  fi

  local expected_decisions
  case "$scenario" in
    nonmax) expected_decisions=$'c1\nc1\nc1\nDONE' ;;
    *) expected_decisions=$'c1\nc1\nc2\nDONE' ;;
  esac
  if [[ "$(cat "$case_dir/decisions")" != "$expected_decisions" ]]; then
    echo "FAIL [$scenario]: Jev decisions changed" >&2
    cat "$case_dir/decisions" >&2
    return 1
  fi

  local expected_c3
  if [[ "$scenario" == tie ]]; then expected_c3='525 comments'; else expected_c3='500 comments'; fi
  if [[ "$scenario" != candidate-count && "$scenario" != long-label ]] && ! jq -e --arg expected_c3 "$expected_c3" '.questions.choice.criteria.c1 == "120 comments" and .questions.choice.criteria.c2 == "525 comments" and .questions.choice.criteria.c3 == $expected_c3 and ((.questions.choice.criteria | has("c4")) | not)' "$case_dir/requests/request-2.json" >/dev/null; then
    echo "FAIL [$scenario]: discussion candidates were not exposed as Choice options" >&2
    return 1
  fi

  for request in "$case_dir"/requests/request-*.json; do
    if ! jq -e '(.state | has("expected_item_id") or has("expected_max_comments") or has("verification") or has("top_level_comments") or has("selected_item_id")) | not' "$request" >/dev/null; then
      echo "FAIL [$scenario]: verifier state leaked into Jev request: $request" >&2
      return 1
    fi
  done

  if [[ ! -e "$TMP_CLOSED" ]]; then
    echo "FAIL [$scenario]: browser cleanup did not run" >&2
    return 1
  fi

  local done_line
  done_line=$(awk '/^decision DONE$/{print NR; exit}' "$case_dir/events")
  if [[ -z "$done_line" ]]; then
    echo "FAIL [$scenario]: DONE was not observed by the fake provider" >&2
    return 1
  fi
  if ! awk -v done="$done_line" 'NR > done && /^snapshot state=3$/{found=1} END{exit !found}' "$case_dir/events"; then
    echo "FAIL [$scenario]: no fresh final snapshot occurred after DONE" >&2
    cat "$case_dir/events" >&2
    return 1
  fi
  expected_evals=$'eval location.href\neval document.title\neval body bounded\neval comments structural'
  actual_evals=$(awk -v done="$done_line" 'NR > done && /^eval /{sub(/ state=.*/, ""); print}' "$case_dir/events")
  if [[ "$actual_evals" != "$expected_evals" ]]; then
    echo "FAIL [$scenario]: post-DONE eval sequence was not exact" >&2
    printf 'expected:\n%s\nactual:\n%s\n' "$expected_evals" "$actual_evals" >&2
    return 1
  fi
  if grep -Fq 'Chosen story' "$root/hn-browser.sh" || grep -Fq 'discussion body' "$root/hn-browser.sh"; then
    echo "FAIL [$scenario]: production contains fixture-only title/body constants" >&2
    return 1
  fi

  case "$scenario" in
    tie|unsafe-links|candidate-count|long-label)
      if [[ "$status" -ne 0 ]]; then
        echo "FAIL [tie]: valid maximum-tie selection failed (status $status)" >&2
        cat "$case_dir/stderr" >&2
        return 1
      fi
      if ! jq -e --arg yesterday "$yesterday" '
        .verification.previous_utc_date == $yesterday and
        .verification.selected_url == "https://news.ycombinator.com/item?id=42" and
        .verification.selected_ref == "e4" and
        .verification.selected_item_id == "42" and
        .verification.selected_comment_count == 525 and
        .verification.maximum_comment_count == 525 and
        .verification.selected_is_maximum == true and
        (.verification.top_level_comments | length >= 1) and
        (.verification.top_level_comments | all((.user | length) > 0 and (.text | length) > 0 and .depth == 0 and (.text | length <= 600)))
      ' "$case_dir/stdout" >/dev/null; then
        echo "FAIL [tie]: verification did not prove actual URL/ref, dynamic date, maximum, and comments" >&2
        cat "$case_dir/stdout" >&2
        return 1
      fi
      ;;
    nonmax)
      if [[ "$status" -eq 0 ]]; then
        echo "FAIL [nonmax]: false verification success for selected non-maximum item" >&2
        cat "$case_dir/stdout" >&2
        return 1
      fi
      ;;
    redirect|wrongday|badtitle|badbody|empty-comments|malformed-comments|missing-depth-comments|oversized-eval)
      if [[ "$status" -eq 0 ]]; then
        echo "FAIL [$scenario]: invalid independent evidence was accepted" >&2
        cat "$case_dir/stdout" >&2
        return 1
      fi
      if [[ "$scenario" == badbody ]] && grep -q '^eval comments structural' "$case_dir/events"; then
        echo "FAIL [badbody]: comments were evaluated before body verification failed" >&2
        return 1
      fi
      ;;
  esac
  if [[ "$scenario" == trace ]]; then
    local trace_lines="$case_dir/trace.ndjson"
    grep -E '^\{"event":' "$case_dir/stderr" >"$trace_lines" || true
    if ! jq -s -e '
      ([.[] | select(.event == "decision")] | length) == 4 and
      ([.[] | select(.event == "decision")] | all(has("step") and has("choice") and has("label") and has("model") and (.usage.input_tokens | numbers) and (.usage.output_tokens | numbers) and (.latency_ms | numbers))) and
      ([.[] | select(.event == "decision") | .choice] == ["c1", "c1", "c2", "DONE"]) and
      ([.[] | select(.event == "completed")] | length) == 1 and
      ([.[] | select(.event == "completed") | select((.requests | numbers) and (.usage.input_tokens | numbers) and (.usage.output_tokens | numbers) and (.elapsed_ms | numbers) and (.verification.selected_item_id == "42"))] | length) == 1 and
      ([.[] | select(.event == "cleanup" and .closed == true)] | length) == 1
    ' "$trace_lines" >/dev/null; then
      echo "FAIL [trace]: missing bounded decision, completion, verification, token, latency, or cleanup records" >&2
      cat "$case_dir/stderr" >&2
      return 1
    fi
  fi
}

failures=()
for scenario in tie nonmax redirect wrongday badtitle badbody empty-comments malformed-comments missing-depth-comments candidate-count long-label unsafe-links headed stale-choice unknown-choice blocked-choice trace oversized-snapshot oversized-eval malformed-response oversized-response oversized-request open-hang snapshot-hang eval-hang click-hang close-hang jeq-hang; do
  if ! run_case "$scenario"; then
    failures+=("$scenario")
  fi
done
if ((${#failures[@]})); then
  echo "FAIL: first unmet verifier case: ${failures[0]} (all failures: ${failures[*]})" >&2
  exit 1
fi
printf 'PASS verifier acceptance cases\n'
