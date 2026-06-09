#!/usr/bin/env bash
# Build swoop-rendezvous for Linux and assemble a VPS deploy bundle.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$REPO_ROOT/build/bin"
DEPLOY_DIR="$REPO_ROOT/build/deploy"
OUT="$OUT_DIR/swoop-rendezvous"

mkdir -p "$OUT_DIR" "$DEPLOY_DIR"

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

printf '[INFO] Building swoop-rendezvous (linux/amd64)...\n'
go build -ldflags="-s -w" -o "$OUT" "$REPO_ROOT/cmd/swoop-rendezvous"

cp "$OUT" "$DEPLOY_DIR/swoop-rendezvous"
cp "$REPO_ROOT/scripts/deploy/install.sh" "$DEPLOY_DIR/install.sh"
cp "$REPO_ROOT/scripts/deploy/swoop-rendezvous.service" "$DEPLOY_DIR/swoop-rendezvous.service"
chmod +x "$DEPLOY_DIR/install.sh"

printf '[ OK ] binary: %s\n' "$OUT"
printf '[ OK ] deploy bundle: %s\n' "$DEPLOY_DIR"
printf '[INFO] On VPS (as root):\n'
printf '       scp -r build/deploy/* root@HOST:/tmp/swoop-deploy/\n'
printf '       ssh root@HOST "cd /tmp/swoop-deploy && ./install.sh"\n'
printf '[INFO] Service: systemctl status swoop-rendezvous\n'
printf '[INFO] Log file: /var/log/swoop/rendezvous.log\n'
