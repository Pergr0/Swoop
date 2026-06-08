#!/usr/bin/env bash
# Build swoop-rendezvous for Linux (VPS signaling server, no file relay).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$REPO_ROOT/build/bin"
OUT="$OUT_DIR/swoop-rendezvous"

mkdir -p "$OUT_DIR"

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

printf '[INFO] Building swoop-rendezvous (linux/amd64)...\n'
go build -ldflags="-s -w" -o "$OUT" "$REPO_ROOT/cmd/swoop-rendezvous"

printf '[ OK ] %s\n' "$OUT"
printf '[INFO] Run on VPS: %s -addr :53400\n' "$OUT"
