#!/usr/bin/env bash
# Red acceptance test for TASK-0064 phase 2a. It intentionally fails until hn-browser.sh loops.
set -u
root=$(cd "$(dirname "$0")" && pwd); bin=$(mktemp -d); capture="$bin/requests"; mkdir "$capture"
trap 'rm -rf "$bin"' EXIT
cat >"$bin/playwright-cli" <<'SH'
#!/usr/bin/env bash
state_file=${TMP_STATE:?}; state=$(cat "$state_file" 2>/dev/null || echo 0)
case "$*" in
  *snapshot*) case "$state" in
    0) printf '%s\n' '- link "past" [ref=e1]:' '  - /url: "news.php"';;
    1) printf '%s\n' '- link "yesterday" [ref=e2]:' '  - /url: "news.php?day=2026-01-01"';;
    2) printf '%s\n' '- link "Most discussed" [ref=e3]:' '  - /url: "item?id=9"';;
    3) printf '%s\n' '- heading "Chosen story" [ref=e4]';;
  esac;;
  *click*) echo $((state+1)) >"$state_file";;
  *close*) touch "$TMP_CLOSED";;
esac
SH
cat >"$bin/jeq" <<'SH'
#!/usr/bin/env bash
n=$(ls "$CAPTURE" | wc -l | tr -d ' '); input="$CAPTURE/request-$n"; cat >"$input"; echo "FAKE_REQUEST=$input" >&2
case "$n" in
  0|1|2) choice=c1;; 3) choice=DONE;; *) choice=BLOCKED;;
esac
printf '{"answers":{"choice":{"choice":"%s"}}}\n' "$choice"
SH
chmod +x "$bin"/*; printf '0\n' >"$bin/state"; export PATH="$bin:$PATH" TMP_STATE="$bin/state" TMP_CLOSED="$bin/closed" CAPTURE="$capture"
set +e; output="$($root/hn-browser.sh 2>&1)"; status=$?; set -e
if [[ $status -eq 0 ]]; then echo "FAIL: one-cycle script unexpectedly passed route acceptance" >&2; exit 1; fi
count=$(find "$capture" -type f | wc -l | tr -d ' ')
if [[ "$count" -ne 4 ]]; then echo "FAIL: first unmet criterion: expected 4 jeq requests, got $count output=$output" >&2; exit 1; fi
echo "FAIL: route assertions not yet reached: $output" >&2; exit 1
