#!/usr/bin/env bash
set -euo pipefail
: "${JEQ_BIN:=jeq}"; : "${PLAYWRIGHT_BIN:=playwright-cli}"
session="jeq-hn-$RANDOM$$"; tmp="$(mktemp -d)"; snapshot="$tmp/snapshot"; archive_snapshot="$tmp/archive"; dated_snapshot="$tmp/dated"; dated_candidates="$tmp/dated-candidates"; candidates="$tmp/candidates"; request="$tmp/request"; response="$tmp/response"
cleanup() { "$PLAYWRIGHT_BIN" -s="$session" close >/dev/null 2>&1 || true; rm -rf "$tmp"; }
trap cleanup EXIT HUP INT TERM
"$PLAYWRIGHT_BIN" -s="$session" open https://news.ycombinator.com/ >/dev/null
url=https://news.ycombinator.com/; recent='[]'
for step in 1 2 3 4; do
  "$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
  "$PLAYWRIGHT_BIN" -s="$session" eval 'location.href' >"$tmp/observe-location"
  "$PLAYWRIGHT_BIN" -s="$session" eval 'document.title' >"$tmp/observe-title"
  observed_url=$(sed -n '/### Result/,$p' "$tmp/observe-location" | tail -1 | tr -d '"')
  observed_title=$(sed -n '/### Result/,$p' "$tmp/observe-title" | tail -1 | tr -d '"')
  [[ "$observed_url" =~ ^https://news\.ycombinator\.com/ ]] || { echo "unsafe observed location" >&2; exit 1; }
  [[ -n "$observed_title" && "$observed_title" != "Error page" ]] || { echo "invalid observed title" >&2; exit 1; }
  if [[ "$step" -eq 2 ]]; then
    expected_day=$(python3 -c 'from datetime import datetime,timedelta,timezone; print((datetime.now(timezone.utc)-timedelta(days=1)).date())')
    [[ "$observed_url" == https://news.ycombinator.com/front*"day=$expected_day"* ]] || { echo "wrong archive day" >&2; exit 1; }
  fi
  url="$observed_url"
  export OBSERVED_URL="$observed_url" OBSERVED_TITLE="$observed_title"
  [[ "$step" -eq 2 ]] && cp "$snapshot" "$archive_snapshot"
  [[ "$step" -eq 3 ]] && cp "$snapshot" "$dated_snapshot"
  : >"$candidates"
  python3 - "$snapshot" >"$candidates" <<'PY'
import re, sys
from urllib.parse import urljoin, urlparse, parse_qs
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
    if 'comment' in m.group(1).lower() and (parsed.path != '/item' or not re.fullmatch(r'[0-9]+', (parse_qs(parsed.query).get('id') or [''])[0])): continue
    n+=1; print(f'c{n}\t{m.group(2)}\t{m.group(1)}\t{urljoin("https://news.ycombinator.com/",href)}')
PY
  [[ "$step" -eq 3 ]] && cp "$candidates" "$dated_candidates"
  jq -n --rawfile snap "$snapshot" --slurpfile rows <(jq -Rn '[inputs|split("\t")|{id:.[0],ref:.[1],label:.[2],url:.[3]}]' "$candidates") --arg url "$url" --arg recent "$recent" --argjson step "$step" '{model:"jev-latest",state:{goal:(env.JEQ_GOAL // "Navigate to yesterday then the most-discussed discussion"),current_date:(now|todate),current_url:$url,observed_url:(env.OBSERVED_URL // $url),title:(env.OBSERVED_TITLE // ""),visible_text:($snap|.[0:12000]),recent_decisions:$recent,step:$step},questions:{choice:{type:"choice",instructions:"Choose one current safe link, DONE, or BLOCKED. Do not invent IDs.",criteria:(($rows[0]|map({key:.id,value:.label})|from_entries)+{DONE:"Finish only after independent verification",BLOCKED:"Stop safely"})}}}' >"$request"
  "$JEQ_BIN" ask --request - <"$request" >"$response"
  id="$(jq -er '.answers.choice.choice // .answers.operation.choice' "$response")"
  if [[ "$id" == DONE ]]; then
    "$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
    eval_result="$tmp/eval"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'location.href' >"$tmp/location"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'document.title' >"$tmp/title"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'document.body.innerText.slice(0,12000)' >"$tmp/body"
    "$PLAYWRIGHT_BIN" -s="$session" eval 'JSON.stringify(Array.from(document.querySelectorAll("tr.comtr")).filter(function(row){var indent=row.querySelector("td.ind img");return indent && Number(indent.getAttribute("width"))===0;}).slice(0,5).map(function(row){return {depth:0,user:row.querySelector("a.hnuser")?.textContent?.trim(),text:row.querySelector("div.commtext")?.textContent?.trim()?.slice(0,600)}}))' >"$tmp/comments"
    python3 - "$archive_snapshot" "$dated_snapshot" "$dated_candidates" "$snapshot" "$url" "$tmp/location" "$tmp/title" "$tmp/body" "$tmp/comments" <<'PY'
import json, re, sys
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urlparse
archive=open(sys.argv[1]).read(); old=open(sys.argv[2]).read(); candidate_file=sys.argv[3]; final=open(sys.argv[4]).read(); selected=sys.argv[5]
location = open(sys.argv[6]).read().split('### Result',1)[-1].strip().strip('"')
title = open(sys.argv[7]).read().split('### Result',1)[-1].strip().strip('"')
body = open(sys.argv[8]).read().split('### Result',1)[-1].strip().strip('"')
comments_raw = open(sys.argv[9]).read().split('### Result',1)[-1].strip()
item_refs={line.split('\t')[1] for line in open(candidate_file) if '/item?id=' in line}
counts=[]
for line in old.splitlines():
    m=re.search(r'link "([0-9]+) comments".*ref=(e\d+)', line)
    if m and m.group(2) in item_refs: counts.append((m.group(2), int(m.group(1))))
if location != selected or 'Hacker News' not in title: raise SystemExit('independent URL/title verification failed')
selected_id=(parse_qs(urlparse(selected).query).get('id') or [''])[0]
selected_ref=next((line.split('\t')[1] for line in open(candidate_file) if (parse_qs(urlparse(line.rstrip().split('\t')[-1]).query).get('id') or [''])[0] == selected_id), '')
selected_count=next((n for ref,n in counts if ref == selected_ref), 0)
maximum=max((n for _,n in counts), default=0)
yesterday=(datetime.now(timezone.utc)-timedelta(days=1)).date().isoformat()
archive_day=next(iter(re.findall(r'front\?day=([0-9]{4}-[0-9]{2}-[0-9]{2})', archive)), '')
if archive_day != yesterday: raise SystemExit('archive day mismatch')
if not re.search(r'\S+ \| Hacker News$', title) or not body.strip() or 'error page' in body.lower(): raise SystemExit('discussion evidence mismatch')
try: comments=json.loads(comments_raw)
except Exception: raise SystemExit('malformed comments')
if not comments: raise SystemExit('empty comments')
if any(not isinstance(x,dict) or not isinstance(x.get('depth'),(int,float)) or x.get('depth') != 0 or not str(x.get('user','')).strip() or not str(x.get('text','')).strip() for x in comments): raise SystemExit('invalid comments')
comments=[{'user':str(x['user']),'text':str(x['text'])[:600],'depth':0} for x in comments[:5]]
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
