#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PREFIX=${PREFIX:-"$HOME/.local"}

say() { printf '%s\n' "==> $*"; }
fail() { printf '%s\n' "error: $*" >&2; exit 1; }

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    fail "administrative privileges are required to install build dependencies"
  fi
}

install_linux_deps() {
  if command -v apt-get >/dev/null 2>&1; then
    say "Installing Go, GDAL/PROJ and build dependencies with apt"
    as_root apt-get update
    as_root apt-get install -y golang-go build-essential pkg-config libgdal-dev libproj-dev gdal-bin ca-certificates
  elif command -v dnf >/dev/null 2>&1; then
    say "Installing Go, GDAL/PROJ and build dependencies with dnf"
    as_root dnf install -y golang gcc gcc-c++ make pkgconf-pkg-config gdal gdal-devel proj proj-devel ca-certificates
  elif command -v pacman >/dev/null 2>&1; then
    say "Installing Go, GDAL/PROJ and build dependencies with pacman"
    as_root pacman -Sy --needed --noconfirm go base-devel pkgconf gdal proj ca-certificates
  else
    fail "unsupported Linux package manager; install Go >=1.23, GDAL >=3.6, PROJ, pkg-config and a C/C++ compiler"
  fi
}

install_macos_deps() {
  command -v brew >/dev/null 2>&1 || fail "Homebrew is required on macOS: https://brew.sh"
  say "Installing Go, GDAL/PROJ and pkg-config with Homebrew"
  brew install go pkg-config gdal
}

case "$(uname -s)" in
  Linux) install_linux_deps ;;
  Darwin) install_macos_deps ;;
  *) fail "source installer supports Linux and macOS; use scripts/install.ps1 for a prebuilt Windows release" ;;
esac

command -v go >/dev/null 2>&1 || fail "Go was not found after dependency installation"
command -v pkg-config >/dev/null 2>&1 || fail "pkg-config was not found"
pkg-config --exists gdal || fail "GDAL development files were not found by pkg-config"
pkg-config --exists proj || fail "PROJ development files were not found by pkg-config"

GO_VERSION=$(go env GOVERSION | sed 's/^go//')
GO_MAJOR=$(printf '%s' "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(printf '%s' "$GO_VERSION" | cut -d. -f2 | sed 's/[^0-9].*$//')
if [ "$GO_MAJOR" -lt 1 ] || { [ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 23 ]; }; then
  fail "Go >=1.23 is required; found $GO_VERSION"
fi

say "Building and testing go-iucn-eoo-aoo from source"
cd "$ROOT"
export CGO_ENABLED=1
export PROJ_NETWORK=OFF
go mod download
go test ./...
mkdir -p bin
go build -buildvcs=false -trimpath -o bin/eoo-aoo ./cmd/eoo-aoo

say "Installing eoo-aoo to $PREFIX/bin"
mkdir -p "$PREFIX/bin"
cp bin/eoo-aoo "$PREFIX/bin/eoo-aoo"
chmod 0755 "$PREFIX/bin/eoo-aoo"

case ":$PATH:" in
  *":$PREFIX/bin:"*) ;;
  *) printf '\n%s\n' "Add $PREFIX/bin to PATH to call eoo-aoo from anywhere." ;;
esac

"$PREFIX/bin/eoo-aoo" version
say "Source installation complete"
