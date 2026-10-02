#!/bin/sh
# HookReplay CLI installer.
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/dataprojectswithMJ/HookReplay/main/install.sh | sh
#
# Downloads the right binary for your OS/arch from GitHub Releases and installs
# it to /usr/local/bin (with sudo if needed) or ~/.local/bin otherwise.
set -e

REPO="dataprojectswithMJ/HookReplay"
VERSION="${VERSION:-latest}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "hookreplay: unsupported architecture '$ARCH'" >&2; exit 1 ;;
esac
case "$OS" in
  darwin|linux) ;;
  *) echo "hookreplay: unsupported OS '$OS' (use the Windows zip from the releases page)" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
fi
VERSION="${VERSION#v}"

TARBALL="hookreplay_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/v${VERSION}/${TARBALL}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "hookreplay: downloading v$VERSION ($OS/$ARCH)…"
curl -fsSL "$URL" -o "$TMPDIR/$TARBALL"

# Verify the SHA-256 checksum (best-effort; falls back gracefully).
if curl -fsSL "https://github.com/$REPO/releases/download/v${VERSION}/checksums.txt" \
    -o "$TMPDIR/checksums.txt" 2>/dev/null; then
  EXPECTED="$(grep "  $TARBALL\$" "$TMPDIR/checksums.txt" | awk '{print $1}')"
  if [ -n "$EXPECTED" ]; then
    ACTUAL="$( (sha256sum "$TMPDIR/$TARBALL" 2>/dev/null || shasum -a 256 "$TMPDIR/$TARBALL") | awk '{print $1}')"
    if [ "$EXPECTED" != "$ACTUAL" ]; then
      echo "hookreplay: checksum mismatch — aborting" >&2
      exit 1
    fi
  fi
fi

tar -xzf "$TMPDIR/$TARBALL" -C "$TMPDIR"

DEST="/usr/local/bin"
if [ ! -w "$DEST" ]; then
  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "$TMPDIR/hookreplay" "$DEST/hookreplay"
  else
    DEST="$HOME/.local/bin"
    mkdir -p "$DEST"
    install -m 755 "$TMPDIR/hookreplay" "$DEST/hookreplay"
  fi
else
  install -m 755 "$TMPDIR/hookreplay" "$DEST/hookreplay"
fi

echo "hookreplay: installed to $DEST/hookreplay"
if ! command -v hookreplay >/dev/null 2>&1; then
  echo "hookreplay: add $DEST to your PATH"
fi
hookreplay --version
