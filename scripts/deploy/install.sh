#!/usr/bin/env bash
# Install or upgrade swoop-rendezvous on a Linux VPS (systemd).
# Run as root from the deploy bundle directory (build/deploy/ on the build host).
set -euo pipefail

INSTALL_DIR=/opt/swoop
LOG_DIR=/var/log/swoop
SERVICE_NAME=swoop-rendezvous
UNIT_PATH=/etc/systemd/system/${SERVICE_NAME}.service

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_SRC="$SCRIPT_DIR/swoop-rendezvous"
UNIT_SRC="$SCRIPT_DIR/swoop-rendezvous.service"

if [[ "$(id -u)" -ne 0 ]]; then
	printf '[ERR ] Run as root: sudo %s\n' "$0" >&2
	exit 1
fi

if [[ ! -f "$BIN_SRC" ]]; then
	printf '[ERR ] Missing binary: %s\n' "$BIN_SRC" >&2
	exit 1
fi
if [[ ! -f "$UNIT_SRC" ]]; then
	printf '[ERR ] Missing unit file: %s\n' "$UNIT_SRC" >&2
	exit 1
fi

if ! id swoop &>/dev/null; then
	useradd --system --no-create-home --shell /usr/sbin/nologin swoop
	printf '[INFO] Created system user swoop\n'
fi

install -d -m 755 "$INSTALL_DIR"
install -m 755 "$BIN_SRC" "$INSTALL_DIR/swoop-rendezvous"

install -d -m 755 "$LOG_DIR"
touch "$LOG_DIR/rendezvous.log"
chown swoop:swoop "$LOG_DIR" "$LOG_DIR/rendezvous.log"
chmod 644 "$LOG_DIR/rendezvous.log"

install -m 644 "$UNIT_SRC" "$UNIT_PATH"

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

printf '[ OK ] %s installed to %s\n' "$SERVICE_NAME" "$INSTALL_DIR"
printf '[ OK ] logs: %s/rendezvous.log\n' "$LOG_DIR"
printf '[INFO] Status: systemctl status %s\n' "$SERVICE_NAME"
printf '[INFO] Follow: tail -f %s/rendezvous.log\n' "$LOG_DIR"
