#!/usr/bin/env bash
set -euo pipefail

REPO="${VEFR_REPO:-project-cvsa/vefr}"
VERSION="${VEFR_VERSION:-latest}"
PREFIX="${VEFR_PREFIX:-/usr/local/bin}"
CONFIG="${VEFR_CONFIG:-/etc/vefr/config.toml}"
SUDO=""

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '\033[2m%s\033[0m\n' "$*"; }
success() { printf '\033[1;32m%s\033[0m\n' "$*"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) [[ $# -ge 2 ]] || die "--version requires a value"; VERSION="$2"; shift 2 ;;
    --prefix) [[ $# -ge 2 ]] || die "--prefix requires a value"; PREFIX="$2"; shift 2 ;;
    --config) [[ $# -ge 2 ]] || die "--config requires a value"; CONFIG="$2"; shift 2 ;;
    --help|-h) printf 'Usage: install.sh [--version TAG] [--prefix DIR] [--config PATH]\n'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ "$(uname -s)" == Linux ]] || die "only Linux is supported"
case "$(uname -m)" in
  x86_64|amd64) ARCH=x86_64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac
command -v curl >/dev/null || die "curl is required"
command -v tar >/dev/null || die "tar is required"
if [[ $EUID -ne 0 ]]; then command -v sudo >/dev/null || die "sudo is required"; SUDO=sudo; fi

if [[ "$VERSION" == latest ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p' | head -n 1)"
  [[ -n "$VERSION" ]] || die "failed to determine the latest release"
fi
BASE="https://github.com/$REPO/releases/download/$VERSION"
ASSET="vefr_${VERSION#v}_Linux_${ARCH}.tar.gz"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT
ARCHIVE="$TMPDIR/vefr.tar.gz"
CHECKSUMS="$TMPDIR/checksums.txt"
info "Downloading vefr $VERSION ($ARCH)..."
curl -fL --progress-bar "$BASE/$ASSET" -o "$ARCHIVE" || die "failed to download $BASE/$ASSET"
curl -fsSL "$BASE/checksums.txt" -o "$CHECKSUMS" || die "failed to download checksums"
EXPECTED="$(grep "  $ASSET$" "$CHECKSUMS" | awk '{print $1}')"
[[ -n "$EXPECTED" ]] || die "checksum entry not found for $ASSET"
if command -v sha256sum >/dev/null; then ACTUAL="$(sha256sum "$ARCHIVE" | awk '{print $1}')"; else ACTUAL="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"; fi
[[ "$ACTUAL" == "$EXPECTED" ]] || die "checksum verification failed"

tar -xzf "$ARCHIVE" -C "$TMPDIR"
$SUDO install -d -m 0755 "$PREFIX"
$SUDO install -m 0755 "$TMPDIR/vefr" "$PREFIX/vefr"
$SUDO "$PREFIX/vefr" systemd install --config "$CONFIG"
if $SUDO test -e "$CONFIG" || $SUDO test -L "$CONFIG"; then
  info "Preserving existing configuration at $CONFIG"
else
  $SUDO install -m 0640 -o root -g vefr "$TMPDIR/config.example.toml" "$CONFIG"
fi
success "vefr $VERSION was installed to $PREFIX/vefr"
info "Review $CONFIG, then run: sudo systemctl enable --now vefr"
