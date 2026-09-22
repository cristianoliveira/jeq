#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")" && pwd); bin=$(mktemp -d); trap 'rm -rf "$bin"' EXIT
cat >"$bin/playwright-cli" <<'SH'
#!/usr/bin/env bash
if [[ "$*" == *snapshot* ]]; then printf '%s\n' '- link "Yesterday" [ref=e1] [cursor=pointer]:' '  - /url: "news.php?day=2026-01-01"'; elif [[ "$*" == *close* ]]; then touch "$TMP_CLOSE"; else touch "$TMP_CLICK"; fi
SH
cat >"$bin/jeq" <<'SH'
#!/usr/bin/env bash
cat >/dev/null; printf '%s\n' '{"answers":{"choice":{"choice":"c1"}}}'
SH
chmod +x "$bin"/*; export PATH="$bin:$PATH" TMP_CLOSE="$bin/closed" TMP_CLICK="$bin/clicked"
"$root/hn-browser.sh" >/dev/null
[[ -e "$TMP_CLICK" && -e "$TMP_CLOSE" ]]
cat >"$bin/jeq" <<'SH'
#!/usr/bin/env bash
cat >/dev/null; printf '%s\n' '{"answers":{"choice":{"choice":"c999"}}}'
SH
if "$root/hn-browser.sh" >/dev/null 2>&1; then exit 1; fi
[[ -e "$TMP_CLOSE" ]]
printf 'PASS fake cli safe click and stale rejection\n'
