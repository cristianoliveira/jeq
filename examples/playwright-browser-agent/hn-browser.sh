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
python3 - "$snapshot" >"$candidates" <<'PY'
import re, sys
from urllib.parse import urljoin, urlparse
lines=open(sys.argv[1], encoding="utf-8").read().splitlines(); n=0
for i,line in enumerate(lines):
    m=re.search(r'- link "([^"]*)".*\[ref=(e\d+)\]', line)
    if not m: continue
    url_match=None
    for child in lines[i+1:i+8]:
        if len(child)-len(child.lstrip()) <= len(line)-len(line.lstrip()): break
        u=re.search(r'/url: "([^"]+)"', child)
        if u: url_match=u.group(1); break
    if not url_match: continue
    url=urljoin('https://news.ycombinator.com/', url_match); parsed=urlparse(url)
    path=parsed.path.lower()
    if parsed.scheme != 'https' or parsed.hostname != 'news.ycombinator.com' or parsed.username or parsed.password: continue
    if any(word in path for word in ('login','logout','submit','vote','hide','reply')): continue
    n+=1; print(f'c{n}\t{m.group(2)}\t{m.group(1)}\t{url}')
PY
while IFS=$'\t' read -r id ref label url; do printf '%s\t%s\t%s\t%s\n' "$id" "$ref" "$label" "$url"; done <"$candidates" >"$tmp/candidates.safe"; mv "$tmp/candidates.safe" "$candidates"
jq -n --rawfile snap "$snapshot" --slurpfile rows <(jq -Rn '[inputs|split("\t")|{id:.[0],ref:.[1],label:.[2]}]' "$candidates") '{model:"jev-latest",state:{goal:(env.JEQ_GOAL // "Navigate to yesterday then the most-discussed discussion"),current_date:(now|todate),snapshot:($snap|.[0:12000])},questions:{choice:{type:"choice",instructions:"Choose one current safe link to advance the goal, or DONE/BLOCKED. Do not invent IDs.",criteria:(( $rows[0] | map({key:.id,value:.label}) | from_entries)+{DONE:"Finish only when independently verified",BLOCKED:"Stop safely"})}}}' >"$request"
"$JEQ_BIN" ask --request - <"$request" >"$response"
id="$(jq -er '.answers.choice.choice // .answers.operation.choice' "$response")"
[[ "$id" =~ ^c[0-9]+$ ]] || { echo "terminal or invalid choice requires independent verification" >&2; exit 1; }
line="$(awk -F '\t' -v id="$id" '$1==id{print; exit}' "$candidates")"; [[ -n "$line" ]] || { echo "stale candidate" >&2; exit 1; }
ref="$(cut -f2 <<<"$line")"; url="$(cut -f4 <<<"$line")"; [[ "$url" =~ ^https://news\.ycombinator\.com/ ]] || exit 1
"$PLAYWRIGHT_BIN" -s="$session" click "$ref"
printf 'choice=%s ref=%s\n' "$id" "$ref"
