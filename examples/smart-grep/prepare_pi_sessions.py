#!/usr/bin/env python3
"""Prepare privacy-preserving Smartgrep candidates from Pi session JSONL files."""
import argparse, json, os, sys, time
from collections import Counter, defaultdict

TAXONOMY = {
    "tool_failure": ("toolResult", "isError"),
    "assistant_error": ("assistant", "errorMessage"),
}
DEFAULT_ROOT = os.path.expanduser("~/.pi/agent/sessions")

def classify(obj):
    if isinstance(obj, dict):
        for key, value in obj.items():
            if key == "errorMessage" and isinstance(value, str): return "assistant_error"
            if key == "toolResult" and isinstance(value, dict) and value.get("isError") is True: return "tool_failure"
            found = classify(value)
            if found: return found
    elif isinstance(obj, list):
        for value in obj:
            found = classify(value)
            if found: return found
    return None

def main(argv=None):
    p=argparse.ArgumentParser(description="Create redacted Pi-session error candidates")
    p.add_argument("--root", default=DEFAULT_ROOT); p.add_argument("--days", type=float, default=7)
    p.add_argument("--since", type=float); p.add_argument("--until", type=float)
    p.add_argument("--max-files", type=int, default=1000); p.add_argument("--max-bytes", type=int, default=50*1024*1024)
    p.add_argument("--max-candidates", type=int, default=29); p.add_argument("--max-payload-bytes", type=int, default=12*1024)
    a=p.parse_args(argv); now=time.time(); since=a.since if a.since is not None else now-a.days*86400; until=a.until if a.until is not None else now
    files=[]; total=0
    for base, _, names in os.walk(a.root):
        for name in names:
            if not name.endswith(".jsonl"): continue
            path=os.path.join(base,name); st=os.stat(path)
            if since <= st.st_mtime <= until: files.append(path)
    files.sort()
    if len(files)>a.max_files: raise ValueError("session file cap exceeded")
    counts=Counter(); sessions=defaultdict(set); total_events=0
    for path in files:
        size=os.path.getsize(path); total += size
        if total>a.max_bytes: raise ValueError("session byte cap exceeded")
        with open(path, encoding="utf-8") as f:
            for line in f:
                try: obj=json.loads(line)
                except json.JSONDecodeError as e: raise ValueError("malformed session JSONL: "+str(e))
                kind=classify(obj)
                if kind:
                    counts[kind]+=1; sessions[kind].add(path); total_events+=1
    rows=[]
    for kind in sorted(TAXONOMY):
        if counts[kind]:
            rows.append({"id":kind,"description":f"{kind}: {counts[kind]} error events across {len(sessions[kind])} session files; repeated events are counted, not wasted turns."})
    rows.append({"id":"none","description":"No classified Pi-session error candidate is a better match."})
    if len(rows)>a.max_candidates+1: raise ValueError("candidate cap exceeded")
    payload=json.dumps(rows, separators=(",",":"))
    if len(payload.encode())>a.max_payload_bytes: raise ValueError("candidate payload cap exceeded")
    sys.stdout.write(payload+"\n")

if __name__=="__main__":
    try: main()
    except (OSError, ValueError) as e: print("prepare_pi_sessions.py: "+str(e), file=sys.stderr); raise SystemExit(2)
