#!/bin/sh
set -eu

REPO=${EOO_AOO_REPO:-lsbjordao/go-iucn-eoo-aoo}
PREFIX=${PREFIX:-"$HOME/.local"}
VERSION=${VERSION:-latest}

say() { printf '%s\n' "==> $*"; }
fail() { printf '%s\n' "error: $*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    fail "administrative privileges are required to install GDAL/PROJ runtime packages"
  fi
}

download() {
  url=$1
  dest=$2
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fL --retry 3 -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "$url" -o "$dest"
  else
    curl -fL --retry 3 "$url" -o "$dest"
  fi
}

install_linux_runtime() {
  if command -v apt-get >/dev/null 2>&1; then
    say "Installing GDAL/PROJ runtime packages with apt"
    as_root apt-get update
    as_root apt-get install -y gdal-bin proj-bin ca-certificates
  else
    fail "prebuilt Linux releases currently target Debian 13 and Ubuntu 24.04; use scripts/install-from-source.sh on other distributions"
  fi
}

install_macos_runtime() {
  command -v brew >/dev/null 2>&1 || fail "Homebrew is required for the GDAL runtime on macOS: https://brew.sh"
  if ! command -v gdalinfo >/dev/null 2>&1; then
    say "Installing GDAL/PROJ runtime with Homebrew"
    brew install gdal
  fi
}

OS=$(uname -s)
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) fail "unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  Linux)
    [ "$ARCH" = amd64 ] || fail "Linux prebuilt releases currently support amd64 only"
    [ -r /etc/os-release ] || fail "cannot identify Linux distribution"
    # shellcheck disable=SC1091
    . /etc/os-release
    case "${ID:-}:${VERSION_ID:-}" in
      debian:13*) PLATFORM="linux-amd64-debian13" ;;
      ubuntu:24.04*) PLATFORM="linux-amd64-ubuntu2404" ;;
      *) fail "no prebuilt release for ${ID:-unknown} ${VERSION_ID:-unknown}; use scripts/install-from-source.sh or the container" ;;
    esac
    install_linux_runtime
    ;;
  Darwin)
    PLATFORM="darwin-$ARCH"
    install_macos_runtime
    ;;
  *) fail "this installer supports Linux and macOS; on Windows use scripts/install.ps1" ;;
esac

ASSET="eoo-aoo-$PLATFORM.tar.gz"
if [ "$VERSION" = latest ]; then
  BASE="https://github.com/$REPO/releases/latest/download"
else
  case "$VERSION" in v*) TAG=$VERSION ;; *) TAG="v$VERSION" ;; esac
  BASE="https://github.com/$REPO/releases/download/$TAG"
fi

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t eoo-aoo)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM

say "Downloading $ASSET"
download "$BASE/$ASSET" "$TMP/$ASSET"
download "$BASE/SHA256SUMS" "$TMP/SHA256SUMS"

EXPECTED=$(awk -v f="$ASSET" '$2 == f || $2 == "*"f {print $1; exit}' "$TMP/SHA256SUMS")
[ -n "$EXPECTED" ] || fail "checksum for $ASSET was not found"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMP/$ASSET" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')
else
  fail "sha256sum or shasum is required for release verification"
fi
[ "$ACTUAL" = "$EXPECTED" ] || fail "SHA-256 mismatch for $ASSET"

mkdir -p "$TMP/unpack"
tar -xzf "$TMP/$ASSET" -C "$TMP/unpack"
BIN=$(find "$TMP/unpack" -type f -name eoo-aoo | head -n 1)
[ -n "$BIN" ] || fail "release archive does not contain eoo-aoo"

mkdir -p "$PREFIX/bin"
cp "$BIN" "$PREFIX/bin/eoo-aoo"
chmod 0755 "$PREFIX/bin/eoo-aoo"

say "Installed eoo-aoo to $PREFIX/bin/eoo-aoo"
case ":$PATH:" in
  *":$PREFIX/bin:"*) ;;
  *) printf '%s\n' "Add $PREFIX/bin to PATH to call eoo-aoo from anywhere." ;;
esac

"$PREFIX/bin/eoo-aoo" version
say "Installation complete; no Go compiler or source build was required"
