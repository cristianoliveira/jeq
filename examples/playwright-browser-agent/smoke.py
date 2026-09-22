#!/usr/bin/env python3
import functools
import http.server
import pathlib
import threading
from agent import BrowserBoundary, Action

root = pathlib.Path(__file__).parent
handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(root))
server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
boundary = BrowserBoundary()
try:
    url = f"http://127.0.0.1:{server.server_port}/fixture.html"
    boundary.open(url)
    boundary.observe()
    search = next(e for e in boundary.elements.values() if e.role == "searchbox")
    boundary.execute(Action("fill", search.ref, "Lisbon"))
    outcome = boundary.inspect()
    if not outcome["url"].endswith("/fixture.html"):
        raise SystemExit(f"unexpected URL: {outcome}")
    print(f"PASS title={outcome['title']} url={outcome['url']}")
finally:
    boundary.close()
    server.shutdown()
    server.server_close()
    thread.join(timeout=2)
