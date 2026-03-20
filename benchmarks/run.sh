#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PARENT_DIR="$(dirname "$SCRIPT_DIR")"
KITS_DIR="$(dirname "$PARENT_DIR")"

kits=(go-wskit go-cachekit go-jwtkit go-httpkit go-logkit)

for kit in "${kits[@]}"; do
    dir="$KITS_DIR/$kit"
    if [ ! -d "$dir" ]; then
        echo "SKIP $kit (not found at $dir)"
        continue
    fi
    echo "=== $kit ==="
    (cd "$dir" && go test -bench=. -benchmem -count=1 -run='^$' ./...)
    echo ""
done
