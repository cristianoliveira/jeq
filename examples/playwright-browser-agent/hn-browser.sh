#!/usr/bin/env bash
set -euo pipefail

# One visible CLI-first observe -> ask -> validate -> click cycle.
: "${JEQ_BIN:=jeq}"; : "${PLAYWRIGHT_BIN:=playwright-cli}"
session="jeq-hn-$RANDOM$$"; tmp="$(mktemp -d)"; snapshot="$tmp/snapshot"; candidates="$tmp/candidates"; request="$tmp/request"; response="$tmp/response"
cleanup() { "$PLAYWRIGHT_BIN" -s="$session" close >/dev/null 2>&1 || true; rm -rf "$tmp"; }
trap cleanup EXIT HUP INT TERM
"$PLAYWRIGHT_BIN" -s="$session" open https://news.ycombinator.com/ >/dev/null
"$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
: >"$candidates"
# Keep only current same-host HTTP(S) links, and assign opaque IDs owned by this script.
awk '/link / && /\[ref=e[0-9]+\]/ { match($0, /\[ref=(e[0-9]+)\]/, r); if (match($0, /\/url: "([^"]+)"/, u)) print "c" ++n "\t" r[1] "\t" u[1] }' "$snapshot" |
while IFS=$'\t' read -r id ref url; do
  [[ "$url" =~ ^https://news\.ycombinator\.com/ ]] || continue
  [[ "$url" != *'@'* && "$url" != *'javascript:'* && "$url" != *'//'*'//' ]] || continue
  printf '%s\t%s\t%s\n' "$id" "$ref" "$url" >>"$candidates"
done
jq -n --rawfile snap "$snapshot" --slurpfile rows <(jq -Rn '[inputs|split("\t")|{id:.[0],ref:.[1],url:.[2]}]' "$candidates") '{model:"jev-latest",state:{goal:(env.JEQ_GOAL // "Navigate to yesterday then the most-discussed discussion"),current_date:(now|todate),snapshot:($snap|.[0:12000])},questions:{choice:{type:"choice",instructions:"Choose one current safe link to advance the goal, or DONE/BLOCKED. Do not invent IDs.",criteria:(( $rows[0] | map({key:.id,value:.url}) | from_entries)+{DONE:"Finish only when independently verified",BLOCKED:"Stop safely"})}}}' >"$request"
"$JEQ_BIN" ask --request - <"$request" >"$response"
id="$(jq -er '.answers.choice.choice // .answers.operation.choice' "$response")"
[[ "$id" =~ ^c[0-9]+$ ]] || { [[ "$id" == DONE || "$id" == BLOCKED ]] && exit 0; echo "invalid choice" >&2; exit 1; }
line="$(awk -F '\t' -v id="$id" '$1==id{print; exit}' "$candidates")"; [[ -n "$line" ]] || { echo "stale candidate" >&2; exit 1; }
ref="$(cut -f2 <<<"$line")"; url="$(cut -f3 <<<"$line")"; [[ "$url" =~ ^https://news\.ycombinator\.com/ ]] || exit 1
"$PLAYWRIGHT_BIN" -s="$session" click "$ref"
printf 'choice=%s ref=%s\n' "$id" "$ref"
