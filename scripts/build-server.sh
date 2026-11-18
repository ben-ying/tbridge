#!/usr/bin/env bash
set -euo pipefail
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO_BIN=${GO_BIN:-go}
command -v "$GO_BIN" >/dev/null || { echo "Set GO_BIN to Go 1.22+ or install Go." >&2; exit 1; }
mkdir -p "$ROOT/dist"
bash "$ROOT/scripts/prepare-web.sh"
cd "$ROOT/server"
"$GO_BIN" test -count=1 ./...
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
    GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 "$GO_BIN" build -buildvcs=false -trimpath -o "$ROOT/dist/tbridge-${target/\//-}" .
done
