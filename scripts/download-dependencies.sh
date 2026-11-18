#!/usr/bin/env bash
set -euo pipefail
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
command -v npm >/dev/null || { echo "Install Node.js 20+ and npm first." >&2; exit 1; }
cd "$ROOT/web"
npm ci --ignore-scripts --no-audit --no-fund
bash "$ROOT/scripts/prepare-web.sh"
echo "Web assets ready. Run bash scripts/build-server.sh to build the server."
