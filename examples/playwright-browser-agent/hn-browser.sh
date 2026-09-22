#!/usr/bin/env bash
set -euo pipefail
: "${JEQ_BIN:=jeq}"; : "${PLAYWRIGHT_BIN:=playwright-cli}"
session="jeq-hn-$RANDOM$$"; tmp="$(mktemp -d)"; snapshot="$tmp/snapshot"; dated_snapshot="$tmp/dated"; dated_candidates="$tmp/dated-candidates"; candidates="$tmp/candidates"; request="$tmp/request"; response="$tmp/response"
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
  [[ "$step" -eq 3 ]] && cp "$candidates" "$dated_candidates"
  jq -n --rawfile snap "$snapshot" --slurpfile rows <(jq -Rn '[inputs|split("\t")|{id:.[0],ref:.[1],label:.[2],url:.[3]}]' "$candidates") --arg url "$url" --arg recent "$recent" --argjson step "$step" '{model:"jev-latest",state:{goal:(env.JEQ_GOAL // "Navigate to yesterday then the most-discussed discussion"),current_date:(now|todate),current_url:$url,title:"HN",visible_text:($snap|.[0:12000]),recent_decisions:$recent,step:$step},questions:{choice:{type:"choice",instructions:"Choose one current safe link, DONE, or BLOCKED. Do not invent IDs.",criteria:(($rows[0]|map({key:.id,value:.label})|from_entries)+{DONE:"Finish only after independent verification",BLOCKED:"Stop safely"})}}}' >"$request"
  "$JEQ_BIN" ask --request - <"$request" >"$response"
  id="$(jq -er '.answers.choice.choice // .answers.operation.choice' "$response")"
  if [[ "$id" == DONE ]]; then
    "$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
    eval_result="$tmp/eval"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'location.href' >"$tmp/location"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'document.title' >"$tmp/title"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'document.querySelectorAll("tr.comtr")' >"$tmp/comments"
    python3 - "$dated_snapshot" "$dated_candidates" "$snapshot" "$url" "$tmp/location" "$tmp/title" "$tmp/comments" <<'PY'
import json, re, sys
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urlparse
old=open(sys.argv[1]).read(); candidate_file=sys.argv[2]; final=open(sys.argv[3]).read(); selected=sys.argv[4]
location = open(sys.argv[5]).read().split('### Result',1)[-1].strip().strip('"')
title = open(sys.argv[6]).read().split('### Result',1)[-1].strip().strip('"')
comments_raw = open(sys.argv[7]).read().split('### Result',1)[-1].strip()
counts=[]
for line in old.splitlines():
    m=re.search(r'link "([0-9]+) comments".*ref=(e\d+)', line)
    if m: counts.append((m.group(2), int(m.group(1))))
if location != selected or 'Hacker News' not in title: raise SystemExit('independent URL/title verification failed')
selected_id=(parse_qs(urlparse(selected).query).get('id') or [''])[0]
selected_ref=next((line.split('\t')[1] for line in open(candidate_file) if line.rstrip().split('\t')[-1] == selected), '')
selected_count=next((n for ref,n in counts if ref == selected_ref), 0)
maximum=max((n for _,n in counts), default=0)
yesterday=(datetime.now(timezone.utc)-timedelta(days=1)).date().isoformat()
try: comments=json.loads(comments_raw)
except Exception: comments=[]
comments=[{'user':str(x.get('user','')),'text':str(x.get('text',''))[:600],'depth':0} for x in comments if isinstance(x,dict) and x.get('depth',0)==0][:5]
if selected_count != maximum: raise SystemExit('selected discussion is not a maximum')
print(json.dumps({'verification':{'previous_utc_date':yesterday,'selected_url':selected,'selected_ref':selected_ref,'selected_item_id':selected_id,'selected_comment_count':selected_count,'maximum_comment_count':maximum,'selected_is_maximum':True,'top_level_comments':comments}}))
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
