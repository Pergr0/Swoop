#!/usr/bin/env bash
# Package build/bin outputs for GitHub Releases.
# Usage: bash scripts/package-release.sh [version] [bin-dir]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${2:-$REPO_ROOT/build/bin}"
OUT_DIR="$REPO_ROOT/build/release"

if [[ -n "${1:-}" ]]; then
	VERSION="${1#v}"
else
	TAG="$(git -C "$REPO_ROOT" describe --tags --abbrev=0 2>/dev/null || true)"
	if [[ -n "$TAG" ]]; then
		N="$(git -C "$REPO_ROOT" rev-list "${TAG}..HEAD" --count 2>/dev/null || echo 0)"
		SHORT="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
		if [[ "$N" -gt 0 ]]; then
			VERSION="${TAG#v}-build.${SHORT}"
		else
			VERSION="${TAG#v}"
		fi
	else
		VERSION="build.$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
	fi
fi

PREFIX="Swoop-v${VERSION}"
mkdir -p "$OUT_DIR"
rm -f "$OUT_DIR"/${PREFIX}-* "$OUT_DIR"/SHA256SUMS "$OUT_DIR"/RELEASE.txt

stage() {
	local name="$1"
	local d
	d="$(mktemp -d "/tmp/swoop-release-${name}.XXXXXX")"
	echo "$d"
}

if [[ -f "$BIN_DIR/swoop.exe" ]]; then
	d="$(stage win)"
	cp "$BIN_DIR/swoop.exe" "$d/"
	( cd "$d" && zip -q "$OUT_DIR/${PREFIX}-windows-amd64.zip" swoop.exe )
	rm -rf "$d"
	printf '[ OK ] %s\n' "$OUT_DIR/${PREFIX}-windows-amd64.zip"
fi

if [[ -f "$BIN_DIR/swoop" ]]; then
	d="$(stage linux)"
	for f in swoop swoop.png swoop.desktop install-desktop-entry.sh; do
		[[ -f "$BIN_DIR/$f" ]] && cp "$BIN_DIR/$f" "$d/"
	done
	tar -czf "$OUT_DIR/${PREFIX}-linux-amd64.tar.gz" -C "$d" .
	rm -rf "$d"
	printf '[ OK ] %s\n' "$OUT_DIR/${PREFIX}-linux-amd64.tar.gz"
fi

if [[ -d "$BIN_DIR/swoop.app" ]]; then
	( cd "$BIN_DIR" && zip -qr "$OUT_DIR/${PREFIX}-macos-arm64.zip" swoop.app )
	printf '[ OK ] %s\n' "$OUT_DIR/${PREFIX}-macos-arm64.zip"
fi

if [[ -f "$BIN_DIR/swoop-rendezvous" ]]; then
	d="$(stage rendezvous)"
	cp "$BIN_DIR/swoop-rendezvous" "$d/"
	DEPLOY="$REPO_ROOT/build/deploy"
	[[ -d "$DEPLOY" ]] || DEPLOY="$REPO_ROOT/scripts/deploy"
	for f in install.sh swoop-rendezvous.service; do
		[[ -f "$DEPLOY/$f" ]] && cp "$DEPLOY/$f" "$d/"
	done
	tar -czf "$OUT_DIR/${PREFIX}-rendezvous-linux-amd64.tar.gz" -C "$d" .
	rm -rf "$d"
	printf '[ OK ] %s\n' "$OUT_DIR/${PREFIX}-rendezvous-linux-amd64.tar.gz"
fi

( cd "$OUT_DIR" && sha256sum ${PREFIX}-* > SHA256SUMS 2>/dev/null || true )
printf '[ OK ] %s\n' "$OUT_DIR/SHA256SUMS"

cat > "$OUT_DIR/RELEASE.txt" <<EOF
Swoop release ${PREFIX}
Built from: $(git -C "$REPO_ROOT" rev-parse HEAD)
Date (UTC): $(date -u '+%Y-%m-%d %H:%M:%S')

Artifacts:
  ${PREFIX}-windows-amd64.zip       - Windows x64 desktop app
  ${PREFIX}-linux-amd64.tar.gz       - Linux x64 desktop (run install-desktop-entry.sh)
  ${PREFIX}-macos-arm64.zip          - macOS Apple Silicon (swoop.app)
  ${PREFIX}-rendezvous-linux-amd64.tar.gz - VPS signaling server (sudo ./install.sh)

Verify: sha256sum -c SHA256SUMS
Upload: gh release create v${VERSION} build/release/${PREFIX}-* --repo Pergr0/Swoop
EOF
printf '[ OK ] %s\n' "$OUT_DIR/RELEASE.txt"
printf '\nRelease bundle ready in: %s\n' "$OUT_DIR"
