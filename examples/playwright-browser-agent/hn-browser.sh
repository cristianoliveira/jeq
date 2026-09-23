#!/usr/bin/env bash
set -euo pipefail
: "${JEQ_BIN:=jeq}"; : "${PLAYWRIGHT_BIN:=playwright-cli}"; : "${JEQ_SUBPROCESS_TIMEOUT_SECONDS:=30}"
MAX_CANDIDATES=64; MAX_LABEL=256; MAX_VISIBLE=12000; MAX_SNAPSHOT=1048576; MAX_REQUEST=65536; MAX_RESPONSE=65536; MAX_STEPS=4
run_bounded() { timeout --foreground "${JEQ_SUBPROCESS_TIMEOUT_SECONDS}s" "$@"; }
read_eval_string() {
  python3 - "$1" <<'PY'
import json, sys
lines=open(sys.argv[1], encoding='utf-8').read().splitlines()
try: value=json.loads(lines[lines.index('### Result')+1])
except (ValueError, IndexError, json.JSONDecodeError): raise SystemExit('malformed Playwright eval result')
if not isinstance(value, str): raise SystemExit('Playwright eval result is not a string')
print(value)
PY
}
session="jeq-hn-$RANDOM$$"; tmp="$(mktemp -d)"; snapshot="$tmp/snapshot"; archive_snapshot="$tmp/archive"; dated_snapshot="$tmp/dated"; dated_candidates="$tmp/dated-candidates"; candidates="$tmp/candidates"; request="$tmp/request"; response="$tmp/response"
cleanup() { local closed=false; run_bounded "$PLAYWRIGHT_BIN" -s="$session" close >/dev/null 2>&1 && closed=true || true; [[ "${TRACE_EMITTED:-}" != 1 ]] && printf '{"event":"cleanup","closed":%s}\n' "$closed" >&2; TRACE_EMITTED=1; rm -rf "$tmp"; }
trap cleanup EXIT
trap 'exit 143' HUP INT TERM
open_args=(open https://news.ycombinator.com/); [[ "${1:-}" == "--headed" ]] && open_args+=(--headed)
run_bounded "$PLAYWRIGHT_BIN" -s="$session" "${open_args[@]}" >/dev/null
url=https://news.ycombinator.com/; recent='[]'; expected_url=""; trace="$tmp/trace"; started=$(date +%s%3N); requests=0; input_total=0; output_total=0
for step in 1 2 3 4; do
  run_bounded "$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
  [[ $(wc -c <"$snapshot") -le $MAX_SNAPSHOT ]] || { echo "snapshot too large" >&2; exit 1; }
  run_bounded "$PLAYWRIGHT_BIN" -s="$session" eval 'location.href' >"$tmp/observe-location"
  run_bounded "$PLAYWRIGHT_BIN" -s="$session" eval 'document.title' >"$tmp/observe-title"
  observed_url=$(read_eval_string "$tmp/observe-location")
  observed_title=$(read_eval_string "$tmp/observe-title")
  [[ "$observed_url" =~ ^https://news\.ycombinator\.com/ ]] || { echo "unsafe observed location" >&2; exit 1; }
  [[ -z "$expected_url" || "$observed_url" == "$expected_url" || ("$expected_url" == */front && "$observed_url" == "$expected_url"\?day=*) ]] || { echo "navigation redirect mismatch" >&2; exit 1; }
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
        u=re.search(r'/url:\s*"?([^"\s]+)"?', child)
        if u: href=u.group(1); break
    if not href: continue
    parsed=urlparse(urljoin('https://news.ycombinator.com/', href)); path=parsed.path.lower()
    if parsed.scheme != 'https' or parsed.hostname != 'news.ycombinator.com' or parsed.port not in (None,443) or parsed.username or parsed.password: continue
    if any(key in parsed.query.lower() for key in ('submit','vote','hide','reply','delete','logout','upvote','downvote','action=','do=','op=')): continue
    if any(word in path for word in ('login','logout','submit','vote','hide','reply','delete','upvote','downvote')): continue
    if 'comment' in m.group(1).lower() and (parsed.path != '/item' or not re.fullmatch(r'[0-9]+', (parse_qs(parsed.query).get('id') or [''])[0])): continue
    if len(m.group(1)) > 256 or n >= 64: continue
    n+=1; print(f'c{n}\t{m.group(2)}\t{m.group(1)}\t{urljoin("https://news.ycombinator.com/",href)}')
PY
  [[ "$step" -eq 3 ]] && cp "$candidates" "$dated_candidates"
  jq -n --rawfile snap "$snapshot" --slurpfile rows <(jq -Rn '[inputs|split("\t")|{id:.[0],ref:.[1],label:.[2],url:.[3]}]' "$candidates") --arg url "$url" --arg recent "$recent" --argjson step "$step" '{model:"jev-latest",state:{goal:(env.JEQ_GOAL // "Navigate to yesterday then the most-discussed discussion"),current_date:(now|todate),current_url:$url,observed_url:(env.OBSERVED_URL // $url),title:(env.OBSERVED_TITLE // ""),visible_text:($snap|.[0:12000]),recent_decisions:$recent,step:$step},questions:{choice:{type:"choice",instructions:"Choose one current safe link, DONE, or BLOCKED. Do not invent IDs.",criteria:(($rows[0]|map({key:.id,value:.label})|from_entries)+{DONE:"Finish only after independent verification",BLOCKED:"Stop safely"})}}}' >"$request"
  [[ $(wc -c <"$request") -le $MAX_REQUEST ]] || { echo "request too large" >&2; exit 1; }
  ask_started=$(date +%s%3N)
  run_bounded "$JEQ_BIN" ask --request - <"$request" >"$response"
  latency_ms=$(( $(date +%s%3N) - ask_started )); (( latency_ms > 0 )) || latency_ms=1
  [[ $(wc -c <"$response") -le $MAX_RESPONSE ]] || { echo "response too large" >&2; exit 1; }
  id="$(jq -er '.answers.choice.choice // .answers.operation.choice' "$response")"
  requests=$((requests+1)); model=$(jq -r '.model // "unknown"' "$response"); input_tokens=$(jq -er '.usage.input_tokens // .usage.prompt_tokens' "$response"); output_tokens=$(jq -er '.usage.output_tokens // .usage.completion_tokens' "$response"); input_total=$((input_total+input_tokens)); output_total=$((output_total+output_tokens)); label=$(awk -F '\t' -v id="$id" '$1==id{print $3; exit}' "$candidates"); [[ "$id" == DONE ]] && label=DONE; printf '{"event":"decision","step":%d,"choice":"%s","label":"%s","model":"%s","usage":{"input_tokens":%s,"output_tokens":%s},"latency_ms":%s}\n' "$step" "$id" "$label" "$model" "$input_tokens" "$output_tokens" "$latency_ms" >&2
  if [[ "$id" == DONE ]]; then
    run_bounded "$PLAYWRIGHT_BIN" -s="$session" snapshot >"$snapshot"
    eval_result="$tmp/eval"
    run_bounded "$PLAYWRIGHT_BIN" -s="$session" eval 'location.href' >"$tmp/location"
    run_bounded "$PLAYWRIGHT_BIN" -s="$session" eval 'document.title' >"$tmp/title"
    run_bounded "$PLAYWRIGHT_BIN" -s="$session" eval 'document.body.innerText.slice(0,12000)' >"$tmp/body"
    [[ $(wc -c <"$tmp/body") -le 12000 ]] || { echo "body evidence too large" >&2; exit 1; }
    grep -qi 'error page' "$tmp/body" && { echo "body verification failed" >&2; exit 1; }
    python3 - "$dated_snapshot" "$dated_candidates" "$url" <<'PY'
import re,sys
from urllib.parse import urlparse,parse_qs
snap=open(sys.argv[1]).read(); selected=sys.argv[3]; refs={line.split('\t')[1] for line in open(sys.argv[2]) if '/item?id=' in line}; sid=(parse_qs(urlparse(selected).query).get('id') or [''])[0]
counts=[(m.group(2),int(m.group(1))) for l in snap.splitlines() if (m:=re.search(r'link "([0-9]+) comments".*ref=(e\d+)',l)) and m.group(2) in refs]
ref=next((line.split('\t')[1] for line in open(sys.argv[2]) if (parse_qs(urlparse(line.rstrip().split('\t')[-1]).query).get('id') or [''])[0] == sid),''); value=next((n for r,n in counts if r==ref),0)
if not counts or value != max(n for _,n in counts): raise SystemExit('selected discussion is not a maximum')
PY
    run_bounded "$PLAYWRIGHT_BIN" -s="$session" eval 'JSON.stringify(Array.from(document.querySelectorAll("tr.comtr")).filter(function(row){var indent=row.querySelector("td.ind img");return indent && Number(indent.getAttribute("width"))===0;}).slice(0,5).map(function(row){return {depth:0,user:row.querySelector("a.hnuser")?.textContent?.trim(),text:row.querySelector("div.commtext")?.textContent?.trim()?.slice(0,600)}}))' >"$tmp/comments"
    [[ $(wc -c <"$tmp/location") -le 2048 && $(wc -c <"$tmp/title") -le 4096 && $(wc -c <"$tmp/body") -le 12000 && $(wc -c <"$tmp/comments") -le 16384 ]] || { echo "eval artifact too large" >&2; exit 1; }
    python3 - "$archive_snapshot" "$dated_snapshot" "$dated_candidates" "$snapshot" "$url" "$tmp/location" "$tmp/title" "$tmp/body" "$tmp/comments" >"$tmp/verification" <<'PY'
import json, re, sys
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urlparse
archive=open(sys.argv[1]).read(); old=open(sys.argv[2]).read(); candidate_file=sys.argv[3]; final=open(sys.argv[4]).read(); selected=sys.argv[5]
def result(path):
    lines=open(path, encoding='utf-8').read().splitlines()
    try: return json.loads(lines[lines.index('### Result')+1])
    except (ValueError, IndexError, json.JSONDecodeError): raise SystemExit('malformed Playwright eval result')
location=result(sys.argv[6]); title=result(sys.argv[7]); body=result(sys.argv[8]); comments_raw=result(sys.argv[9])
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
try: comments=json.loads(comments_raw) if isinstance(comments_raw, str) else comments_raw
except Exception: raise SystemExit('malformed comments')
if not comments: raise SystemExit('empty comments')
if any(not isinstance(x,dict) or not isinstance(x.get('depth'),(int,float)) or x.get('depth') != 0 or not str(x.get('user','')).strip() or not str(x.get('text','')).strip() for x in comments): raise SystemExit('invalid comments')
comments=[{'user':str(x['user']),'text':str(x['text'])[:600],'depth':0} for x in comments[:5]]
if selected_count != maximum: raise SystemExit('selected discussion is not a maximum')
print(json.dumps({'verification':{'previous_utc_date':yesterday,'selected_url':selected,'selected_ref':selected_ref,'selected_item_id':selected_id,'selected_comment_count':selected_count,'maximum_comment_count':maximum,'selected_is_maximum':True,'top_level_comments':comments}}))
PY
    elapsed=$(( $(date +%s%3N) - started )); verification=$(jq '.verification' "$tmp/verification"); cat "$tmp/verification"; jq -cn --argjson verification "$verification" --argjson requests "$requests" --argjson input "$input_total" --argjson output "$output_total" --argjson elapsed "$elapsed" '{event:"completed",requests:$requests,usage:{input_tokens:$input,output_tokens:$output},elapsed_ms:$elapsed,verification:$verification}' >&2
    exit 0
  fi
  [[ "$id" != BLOCKED && "$id" =~ ^c[0-9]+$ ]] || { echo "invalid or blocked choice" >&2; exit 1; }
  line="$(awk -F '\t' -v id="$id" '$1==id{print; exit}' "$candidates")"; [[ -n "$line" ]] || { echo "stale candidate" >&2; exit 1; }
  ref="$(cut -f2 <<<"$line")"; url="$(cut -f4 <<<"$line")"
  run_bounded "$PLAYWRIGHT_BIN" -s="$session" click "$ref"
  expected_url="$url"
  recent="$(jq -cn --argjson old "$recent" --arg id "$id" '$old+[$id]|.[-8:]')"
done
exit 1
