#!/usr/bin/env bash
#
# Swoop build script for Linux and macOS.
#
# It checks dependencies, installs only what is missing, updates only what is
# outdated (Go/Node by minimum version), builds the app, prints the result and
# the binary location. It can also remove exactly the dependencies it installed.
#
# Usage:
#   scripts/build.sh                 check + install/update + build
#   scripts/build.sh --check-only    check + install/update, no build
#   scripts/build.sh --clean         remove dependencies this script installed
#   scripts/build.sh --help          show this help
#
# Output is plain ASCII on purpose (no special characters).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STATE_FILE="$REPO_ROOT/.swoop-deps.txt"
OUTPUT_NAME="swoop"

MIN_GO_MAJOR=1
MIN_GO_MINOR=21
MIN_NODE_MAJOR=18

OS="$(uname -s)"

ok()   { printf '[ OK ] %s\n' "$*"; }
info() { printf '[INFO] %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*"; }
fail() { printf '[FAIL] %s\n' "$*"; }
step() { printf '\n=== %s ===\n' "$*"; }

usage() {
  sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
}

have() { command -v "$1" >/dev/null 2>&1; }

record() {
  # Record a dependency we installed, so --clean can remove exactly these.
  touch "$STATE_FILE"
  grep -qxF "$1" "$STATE_FILE" 2>/dev/null || printf '%s\n' "$1" >> "$STATE_FILE"
}

detect_pm() {
  if have apt-get; then echo apt
  elif have dnf; then echo dnf
  elif have pacman; then echo pacman
  else echo none
  fi
}

go_ok() {
  have go || return 1
  local v major rest minor
  v="$(go version | awk '{print $3}' | sed 's/^go//')"
  major="${v%%.*}"; rest="${v#*.}"; minor="${rest%%.*}"
  [ "$major" -gt "$MIN_GO_MAJOR" ] && return 0
  [ "$major" -eq "$MIN_GO_MAJOR" ] && [ "$minor" -ge "$MIN_GO_MINOR" ]
}

node_ok() {
  have node || return 1
  local v major
  v="$(node --version | sed 's/^v//')"; major="${v%%.*}"
  [ "$major" -ge "$MIN_NODE_MAJOR" ]
}

ensure_path_go_bin() {
  if have go; then
    local gobin
    gobin="$(go env GOPATH 2>/dev/null)/bin"
    case ":$PATH:" in
      *":$gobin:"*) : ;;
      *) export PATH="$PATH:$gobin" ;;
    esac
  fi
}

# ---------------------------------------------------------------------------
# Dependency checks and installation
# ---------------------------------------------------------------------------

check_only=0
do_clean=0

for arg in "$@"; do
  case "$arg" in
    --check-only) check_only=1 ;;
    --clean)      do_clean=1 ;;
    -h|--help)    usage; exit 0 ;;
    *) fail "unknown argument: $arg"; usage; exit 2 ;;
  esac
done

PM="$(detect_pm)"

clean_deps() {
  step "Cleaning dependencies installed by this script"
  if [ ! -f "$STATE_FILE" ]; then
    warn "no state file ($STATE_FILE); nothing was installed by this script"
    exit 0
  fi
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    case "$line" in
      apt:*)    info "apt remove ${line#apt:}";    sudo apt-get remove -y "${line#apt:}" || warn "failed to remove ${line#apt:}" ;;
      dnf:*)    info "dnf remove ${line#dnf:}";    sudo dnf remove -y "${line#dnf:}" || warn "failed to remove ${line#dnf:}" ;;
      pacman:*) info "pacman -R ${line#pacman:}";  sudo pacman -R --noconfirm "${line#pacman:}" || warn "failed to remove ${line#pacman:}" ;;
      brew:*)   info "brew uninstall ${line#brew:}"; brew uninstall "${line#brew:}" || warn "failed to remove ${line#brew:}" ;;
      go:wails) info "removing wails CLI"; rm -f "$(go env GOPATH 2>/dev/null)/bin/wails" || true ;;
      *) warn "unknown state entry: $line" ;;
    esac
  done < "$STATE_FILE"
  rm -f "$STATE_FILE"
  ok "done; removed dependencies recorded in state file"
}

install_linux() {
  case "$PM" in
    apt)
      sudo apt-get update
      ensure_apt build-essential
      ensure_apt pkg-config
      ensure_apt libgtk-3-dev
      ensure_webkit_apt
      if ! node_ok; then
        info "installing nodejs and npm"
        ensure_apt nodejs
        ensure_apt npm
      else
        ok "Node $(node --version) present"
      fi
      if ! go_ok; then
        info "installing golang-go"
        ensure_apt golang-go
      else
        ok "Go $(go version | awk '{print $3}') present"
      fi
      ;;
    dnf)
      ensure_dnf gcc
      ensure_dnf pkgconf-pkg-config
      ensure_dnf gtk3-devel
      if ! rpm -q webkit2gtk4.1-devel >/dev/null 2>&1 && ! rpm -q webkit2gtk3-devel >/dev/null 2>&1; then
        info "installing webkit2gtk devel"
        sudo dnf install -y webkit2gtk4.1-devel && record "dnf:webkit2gtk4.1-devel" \
          || { sudo dnf install -y webkit2gtk3-devel; record "dnf:webkit2gtk3-devel"; }
      else
        ok "webkit2gtk devel present"
      fi
      node_ok || { ensure_dnf nodejs; ensure_dnf npm; }
      go_ok   || ensure_dnf golang
      ;;
    pacman)
      ensure_pacman base-devel
      ensure_pacman pkgconf
      ensure_pacman gtk3
      ensure_pacman webkit2gtk-4.1
      node_ok || ensure_pacman nodejs
      node_ok || ensure_pacman npm
      go_ok   || ensure_pacman go
      ;;
    none)
      fail "no supported package manager (apt/dnf/pacman) found"
      warn "install manually: C compiler, pkg-config, GTK3 dev, WebKit2GTK 4.1 dev, Node.js, npm, Go 1.21+"
      exit 1
      ;;
  esac
}

ensure_apt() {
  local pkg="$1"
  if dpkg -s "$pkg" >/dev/null 2>&1; then
    ok "$pkg present"
  else
    info "installing $pkg"
    sudo apt-get install -y "$pkg"
    record "apt:$pkg"
  fi
}

ensure_webkit_apt() {
  if dpkg -s libwebkit2gtk-4.1-dev >/dev/null 2>&1 || dpkg -s libwebkit2gtk-4.0-dev >/dev/null 2>&1; then
    ok "WebKit2GTK dev present"
    return
  fi
  local pkg
  if apt-cache show libwebkit2gtk-4.1-dev >/dev/null 2>&1; then
    pkg=libwebkit2gtk-4.1-dev
  else
    pkg=libwebkit2gtk-4.0-dev
  fi
  info "installing $pkg"
  sudo apt-get install -y "$pkg"
  record "apt:$pkg"
}

ensure_dnf() {
  local pkg="$1"
  if rpm -q "$pkg" >/dev/null 2>&1; then
    ok "$pkg present"
  else
    info "installing $pkg"
    sudo dnf install -y "$pkg"
    record "dnf:$pkg"
  fi
}

ensure_pacman() {
  local pkg="$1"
  if pacman -Q "$pkg" >/dev/null 2>&1; then
    ok "$pkg present"
  else
    info "installing $pkg"
    sudo pacman -S --needed --noconfirm "$pkg"
    record "pacman:$pkg"
  fi
}

install_macos() {
  if ! have brew; then
    fail "Homebrew is required on macOS. Install it from https://brew.sh and re-run."
    exit 1
  fi
  if ! xcode-select -p >/dev/null 2>&1; then
    warn "Xcode Command Line Tools not found. Run: xcode-select --install"
  fi
  if go_ok; then
    ok "Go $(go version | awk '{print $3}') present"
  elif have go; then
    info "updating outdated Go via brew"
    brew upgrade go || true
  else
    info "installing Go via brew"
    brew install go
    record "brew:go"
  fi
  if node_ok; then
    ok "Node $(node --version) present"
  elif have node; then
    info "updating outdated Node via brew"
    brew upgrade node || true
  else
    info "installing Node via brew"
    brew install node
    record "brew:node"
  fi
}

ensure_wails() {
  ensure_path_go_bin
  if have wails; then
    ok "Wails CLI present ($(wails version 2>/dev/null | head -n1))"
    return
  fi
  if ! have go; then
    fail "Go is required to install the Wails CLI"
    exit 1
  fi
  info "installing Wails CLI via go install"
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  record "go:wails"
  ensure_path_go_bin
}

# Frontend native deps (esbuild) are platform-specific. If node_modules was
# installed on or copied from another OS, wipe it so npm reinstalls cleanly.
ensure_frontend_deps() {
  local nm="$REPO_ROOT/frontend/node_modules"
  local marker="$nm/.swoop-platform"
  if [ -d "$nm" ]; then
    if [ ! -f "$marker" ] || [ "$(cat "$marker" 2>/dev/null)" != "$OS" ]; then
      warn "frontend/node_modules looks foreign (built on another OS); removing for a clean install"
      rm -rf "$nm"
    fi
  fi
}

mark_frontend_platform() {
  local nm="$REPO_ROOT/frontend/node_modules"
  [ -d "$nm" ] && printf '%s\n' "$OS" > "$nm/.swoop-platform" || true
}

frontend_needs_install() {
  local fe="$REPO_ROOT/frontend"
  local nm="$fe/node_modules"
  local stamp="$nm/.swoop-install-stamp"
  [ ! -d "$nm" ] && return 0
  [ ! -f "$stamp" ] && return 0
  [ "$fe/package-lock.json" -nt "$stamp" ] && return 0
  return 1
}

frontend_needs_vite() {
  local fe="$REPO_ROOT/frontend"
  [ ! -f "$fe/dist/index.html" ] && return 0
  find "$fe/src" -newer "$fe/dist/index.html" -print -quit 2>/dev/null | grep -q .
}

build_frontend() {
  local fe="$REPO_ROOT/frontend"
  if [ ! -f "$fe/package.json" ]; then
    fail "frontend/package.json missing"
    exit 1
  fi
  if frontend_needs_install; then
    info "installing frontend dependencies (npm ci)"
    (cd "$fe" && npm ci --no-fund --no-audit) || {
      fail "npm ci failed"
      exit 1
    }
    touch "$fe/node_modules/.swoop-install-stamp"
    mark_frontend_platform
  fi
  if frontend_needs_vite; then
    info "building frontend (vite)"
    (cd "$fe" && npm run build) || {
      fail "frontend build failed"
      exit 1
    }
  else
    info "frontend dist is up to date"
  fi
  if [ ! -f "$fe/dist/index.html" ]; then
    fail "frontend build did not produce dist/index.html"
    exit 1
  fi
}

package_linux_assets() {
  case "$OS" in
    Linux) ;;
    *) return 0 ;;
  esac
  local src="$REPO_ROOT/build/appicon.png"
  local bindir="$REPO_ROOT/build/bin"
  local bin="$bindir/swoop"
  if [ ! -f "$bin" ]; then
    return 0
  fi
  if [ ! -f "$src" ]; then
    warn "build/appicon.png missing; skipping Linux launcher assets"
    return
  fi
  info "packaging Linux launcher icon"
  cp -f "$src" "$bindir/swoop.png"
  cat > "$bindir/install-desktop-entry.sh" << 'SCRIPT'
#!/bin/sh
set -e
BINDIR=$(cd "$(dirname "$0")" && pwd)
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
APPS_DIR="$HOME/.local/share/applications"
mkdir -p "$ICON_DIR" "$APPS_DIR"
cp -f "$BINDIR/swoop.png" "$ICON_DIR/swoop.png"
cat > "$APPS_DIR/swoop.desktop" << EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Swoop
GenericName=File Transfer
Comment=Zero-config LAN file transfer
Exec=$BINDIR/swoop
Icon=swoop
Terminal=false
Categories=Network;FileTransfer;Utility;
StartupWMClass=swoop
EOF
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$APPS_DIR" 2>/dev/null || true
fi
echo "Installed menu entry and icon for Swoop"
echo "  binary: $BINDIR/swoop"
echo "  menu:   $APPS_DIR/swoop.desktop"
SCRIPT
  chmod +x "$bindir/install-desktop-entry.sh"
  cat > "$bindir/swoop.desktop" << EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Swoop
GenericName=File Transfer
Comment=Zero-config LAN file transfer
Exec=$bindir/swoop
Icon=$bindir/swoop.png
Terminal=false
Categories=Network;FileTransfer;Utility;
StartupWMClass=swoop
EOF
}

sync_app_icon() {
  # Drop cached platform icon artifacts so each build picks up build/appicon.png.
  # Windows: Wails only writes icon.ico when missing (same as Explorer W otherwise).
  # macOS: Wails regenerates .icns in the .app bundle; drop any stale bundle/icns.
  # Linux: GTK icon is go:embed in main.go (rebuilt with the binary); swoop.png is
  # repackaged into build/bin/ after the build via package_linux_assets.
  local src="$REPO_ROOT/build/appicon.png"
  if [ ! -f "$src" ]; then
    warn "build/appicon.png missing; Wails will use a default icon"
    return
  fi
  case "$OS" in
    Linux)
      info "refreshing Linux icon from build/appicon.png"
      rm -f "$REPO_ROOT/build/bin/swoop.png"
      ;;
    Darwin)
      info "refreshing macOS icon from build/appicon.png"
      rm -f "$REPO_ROOT/build/darwin/iconfile.icns"
      rm -rf "$REPO_ROOT/build/bin/swoop.app"
      ;;
    MINGW*|MSYS*|CYGWIN*)
      info "refreshing Windows icon from build/appicon.png"
      rm -rf "$REPO_ROOT/build/bin"
      rm -f "$REPO_ROOT/build/windows/icon.ico"
      rm -f "$REPO_ROOT"/*-res.syso
      rm -f "$REPO_ROOT/build/windows"/rsrc_*.syso
      info "regenerating build/windows/icon.ico"
      (cd "$REPO_ROOT/scripts/genicon" && SWOOP_ROOT="$REPO_ROOT" go run .) || {
        fail "failed to regenerate icon.ico from build/appicon.png"
        exit 1
      }
      ;;
    *)
      ;;
  esac
}

do_build() {
  step "Building Swoop"
  cd "$REPO_ROOT"
  sync_app_icon
  ensure_frontend_deps
  build_frontend
  local tags=""
  if [ "$OS" = "Linux" ] && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    tags="-tags webkit2_41"
    info "detected WebKit2GTK 4.1, using build tag webkit2_41"
  fi
  info "running: wails build -s -f $tags"
  if wails build -s -f $tags; then
    ok "build succeeded"
    mark_frontend_platform
    package_linux_assets
    show_binary
    return 0
  else
    fail "build failed"
    return 1
  fi
}

show_binary() {
  local bindir="$REPO_ROOT/build/bin"
  step "Build output"
  if [ -d "$bindir" ]; then
    info "location: $bindir"
    ls -lh "$bindir" || true
  else
    warn "expected output directory not found: $bindir"
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

if [ "$do_clean" -eq 1 ]; then
  clean_deps
  exit 0
fi

step "Checking and installing dependencies ($OS, package manager: $PM)"
case "$OS" in
  Linux)  install_linux ;;
  Darwin) install_macos ;;
  *) fail "unsupported OS: $OS"; exit 1 ;;
esac
ensure_wails

if ! go_ok; then
  warn "Go is still older than ${MIN_GO_MAJOR}.${MIN_GO_MINOR}; install a newer Go from https://go.dev/dl/"
fi
if ! node_ok; then
  warn "Node is still older than ${MIN_NODE_MAJOR}; install a newer Node.js (e.g. via nodesource)"
fi

if [ "$check_only" -eq 1 ]; then
  ok "dependencies ready (build skipped: --check-only)"
  exit 0
fi

do_build
