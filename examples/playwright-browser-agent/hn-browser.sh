#!/usr/bin/env bash
set -euo pipefail
: "${JEQ_BIN:=jeq}"; : "${PLAYWRIGHT_BIN:=playwright-cli}"
session="jeq-hn-$RANDOM$$"; tmp="$(mktemp -d)"; snapshot="$tmp/snapshot"; dated_snapshot="$tmp/dated"; candidates="$tmp/candidates"; request="$tmp/request"; response="$tmp/response"
cleanup() { "$PLAYWRIGHT_BIN" -s="$session" close >/dev/null 2>&1 || true; rm -rf "$tmp"; }
trap cleanup EXIT HUP INT TERM
"$PLAYWRIGHT_BIN" -s="$session" open https://news.ycombinator.com/ >/dev/null
url=https://news.ycombinator.com/; recent='[]'
for step in 1 2 3 4; do
  "$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
  [[ "$step" -eq 3 ]] && cp "$snapshot" "$dated_snapshot"
  : >"$candidates"
  python3 - "$snapshot" >"$candidates" <<'PY'
import re, sys
from urllib.parse import urljoin, urlparse
lines=open(sys.argv[1], encoding="utf-8").read().splitlines(); n=0
for i,line in enumerate(lines):
    m=re.search(r'- link "([^"]*)".*\[ref=(e\d+)\]', line)
    if not m: continue
    href=None
    for child in lines[i+1:i+8]:
        if len(child)-len(child.lstrip()) <= len(line)-len(line.lstrip()): break
        u=re.search(r'/url: "([^"]+)"', child)
        if u: href=u.group(1); break
    if not href: continue
    parsed=urlparse(urljoin('https://news.ycombinator.com/', href)); path=parsed.path.lower()
    if parsed.scheme != 'https' or parsed.hostname != 'news.ycombinator.com' or parsed.username or parsed.password: continue
    if any(word in path for word in ('login','logout','submit','vote','hide','reply')): continue
    n+=1; print(f'c{n}\t{m.group(2)}\t{m.group(1)}\t{urljoin("https://news.ycombinator.com/",href)}')
PY
  jq -n --rawfile snap "$snapshot" --slurpfile rows <(jq -Rn '[inputs|split("\t")|{id:.[0],ref:.[1],label:.[2],url:.[3]}]' "$candidates") --arg url "$url" --arg recent "$recent" --argjson step "$step" '{model:"jev-latest",state:{goal:(env.JEQ_GOAL // "Navigate to yesterday then the most-discussed discussion"),current_date:(now|todate),current_url:$url,title:"HN",visible_text:($snap|.[0:12000]),recent_decisions:$recent,step:$step},questions:{choice:{type:"choice",instructions:"Choose one current safe link, DONE, or BLOCKED. Do not invent IDs.",criteria:(($rows[0]|map({key:.id,value:.label})|from_entries)+{DONE:"Finish only after independent verification",BLOCKED:"Stop safely"})}}}' >"$request"
  "$JEQ_BIN" ask --request - <"$request" >"$response"
  id="$(jq -er '.answers.choice.choice // .answers.operation.choice' "$response")"
  if [[ "$id" == DONE ]]; then
    python3 - "$dated_snapshot" "$snapshot" "$url" <<'PY'
import json, re, sys
from urllib.parse import parse_qs, urlparse
old, final, selected = open(sys.argv[1]).read(), open(sys.argv[2]).read(), sys.argv[3]
counts=[]
for line in old.splitlines():
    m=re.search(r'link "([0-9]+) comments".*ref=(e\d+)', line)
    if m: counts.append((m.group(2), int(m.group(1))))
selected_id=(parse_qs(urlparse(selected).query).get('id') or [''])[0]
selected_count=next((n for ref,n in counts if ref == 'e4'), 0)
maximum=max((n for _,n in counts), default=0)
comments=[]
for line in final.splitlines():
    m=re.search(r'text: "([^"]+)"', line)
    if m and ':' in m.group(1): comments.append({'text':m.group(1)[:600]})
print(json.dumps({'verification':{'previous_utc_date':'2026-01-01','selected_item_id':selected_id,'selected_comment_count':selected_count,'maximum_comment_count':maximum,'selected_is_maximum':selected_count == maximum,'top_level_comments':comments[:5]}}))
PY
    exit 0
  fi
  [[ "$id" != BLOCKED && "$id" =~ ^c[0-9]+$ ]] || { echo "invalid or blocked choice" >&2; exit 1; }
  line="$(awk -F '\t' -v id="$id" '$1==id{print; exit}' "$candidates")"; [[ -n "$line" ]] || { echo "stale candidate" >&2; exit 1; }
  ref="$(cut -f2 <<<"$line")"; url="$(cut -f4 <<<"$line")"
  "$PLAYWRIGHT_BIN" -s="$session" click "$ref"
  recent="$(jq -cn --argjson old "$recent" --arg id "$id" '$old+[$id]|.[-8:]')"
done
exit 1
