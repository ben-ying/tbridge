#!/usr/bin/env bash
set -euo pipefail
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MODULES="$ROOT/web/node_modules"
if [[ ! -f "$MODULES/@xterm/xterm/lib/xterm.js" || ! -f "$MODULES/@xterm/addon-fit/lib/addon-fit.js" ]]; then
    echo "Run scripts/download-dependencies.sh first to download the pinned terminal packages." >&2
    exit 1
fi
# Bundle the browser terminal sources and pinned dependencies into the Go embed tree.
TARGET="$ROOT/server/ui/terminal"
mkdir -p "$TARGET/vendor"
cp "$ROOT/web/terminal/terminal.js" "$TARGET/terminal.js"
cp "$ROOT/web/terminal/terminal.css" "$TARGET/terminal.css"
cp "$MODULES/@xterm/xterm/lib/xterm.js" "$TARGET/vendor/xterm.js"
cp "$MODULES/@xterm/xterm/css/xterm.css" "$TARGET/vendor/xterm.css"
cp "$MODULES/@xterm/addon-fit/lib/addon-fit.js" "$TARGET/vendor/addon-fit.js"
cp "$MODULES/@xterm/xterm/LICENSE" "$TARGET/vendor/xterm-LICENSE"
cp "$MODULES/@xterm/addon-fit/LICENSE" "$TARGET/vendor/addon-fit-LICENSE"
