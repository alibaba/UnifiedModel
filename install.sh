#!/bin/sh
# install.sh — install UnifiedModel CLIs (umctl, umodel-server, umodel-mcp).
#
#   curl -fsSL https://raw.githubusercontent.com/alibaba/UnifiedModel/main/install.sh | sh
#
# Environment overrides:
#   UM_VERSION   tag to install (default: latest)
#   UM_BINDIR    install dir   (default: /usr/local/bin, fallback: $HOME/.local/bin)
#   UM_BINS      binaries      (default: "umctl umodel-server umodel-mcp")
#
# Zero-setup demo without installing anything:
#   docker run --rm -p 8080:8080 ghcr.io/alibaba/unifiedmodel:latest
set -eu

REPO="alibaba/UnifiedModel"
PROJECT="unifiedmodel"
: "${UM_VERSION:=latest}"
: "${UM_BINS:=umctl umodel-server umodel-mcp}"

err() { printf 'error: %s\n' "$1" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

have curl || err "curl is required"
have tar || err "tar is required"

# Detect OS.
os="$(uname -s)"
case "$os" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) err "unsupported OS: $os — use the Docker image or download from the Releases page" ;;
esac

# Detect arch.
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) ARCH=amd64 ;;
  arm64 | aarch64) ARCH=arm64 ;;
  *) err "unsupported arch: $arch" ;;
esac

# Resolve version (follow the releases/latest redirect — no jq, no API limits).
if [ "$UM_VERSION" = "latest" ]; then
  VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest" | sed 's#.*/tag/##')"
  [ -n "$VERSION" ] || err "could not resolve the latest version"
else
  VERSION="$UM_VERSION"
fi
# Archive names use {{ .Version }} (no leading 'v').
VER_NOV="$(printf '%s' "$VERSION" | sed 's/^v//')"

ASSET="${PROJECT}_${VER_NOV}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

printf 'Downloading %s/%s\n' "$BASE" "$ASSET"
curl -fsSL "${BASE}/${ASSET}" -o "$tmp/$ASSET" || err "download failed: ${BASE}/${ASSET}"

# Best-effort checksum verification.
if curl -fsSL "${BASE}/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null; then
  if have sha256sum; then sum="$(sha256sum "$tmp/$ASSET" | awk '{print $1}')"
  elif have shasum; then sum="$(shasum -a 256 "$tmp/$ASSET" | awk '{print $1}')"
  else sum=""; fi
  if [ -n "$sum" ]; then
    grep -q "$sum  $ASSET" "$tmp/checksums.txt" || err "checksum mismatch for $ASSET"
    printf 'checksum OK\n'
  fi
fi

tar -xzf "$tmp/$ASSET" -C "$tmp"

# Choose install dir.
if [ -n "${UM_BINDIR:-}" ]; then
  BINDIR="$UM_BINDIR"
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  BINDIR=/usr/local/bin
else
  BINDIR="$HOME/.local/bin"
fi
mkdir -p "$BINDIR"

for b in $UM_BINS; do
  if [ -f "$tmp/$b" ]; then
    install -m 0755 "$tmp/$b" "$BINDIR/$b" 2>/dev/null \
      || { cp "$tmp/$b" "$BINDIR/$b" && chmod 0755 "$BINDIR/$b"; }
    printf 'installed %s -> %s\n' "$b" "$BINDIR/$b"
  fi
done

case ":$PATH:" in
  *":$BINDIR:"*) : ;;
  *) printf '\nNote: %s is not on PATH. Add it with:\n  export PATH="%s:$PATH"\n' "$BINDIR" "$BINDIR" ;;
esac

printf '\nDone. Try: umctl version\n'
printf 'For the zero-setup demo (no install), prefer Docker:\n'
printf '  docker run --rm -p 8080:8080 ghcr.io/%s:latest\n' "$PROJECT"
