#!/usr/bin/env bash
set -euo pipefail

REPO="claytercek/offstage"
BINARY="offstage"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s)
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)       ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  Darwin) ARCH="all" ;;
  Linux) ;;
  *) echo "error: unsupported OS: $OS (use install.ps1 on Windows)" >&2; exit 1 ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "error: could not determine latest version" >&2
  exit 1
fi

VERSION_NO_V="${VERSION#v}"
FILENAME="${BINARY}_${VERSION_NO_V}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $BINARY $VERSION..."
curl -fsSL "$BASE_URL/$FILENAME" -o "$TMP/$FILENAME"
curl -fsSL "$BASE_URL/checksums.txt" -o "$TMP/checksums.txt"

echo "Verifying checksum..."
cd "$TMP"
if command -v sha256sum &>/dev/null; then
  grep "$FILENAME" checksums.txt | sha256sum -c -
elif command -v shasum &>/dev/null; then
  grep "$FILENAME" checksums.txt | shasum -a 256 -c -
else
  echo "warning: sha256sum/shasum not found, skipping checksum verification" >&2
fi
cd - >/dev/null

tar -xzf "$TMP/$FILENAME" -C "$TMP"

if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo install -m 755 "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  install -m 755 "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

if [ "$OS" = "Darwin" ]; then
  xattr -dr com.apple.quarantine "$INSTALL_DIR/$BINARY" 2>/dev/null || true
fi

echo "$BINARY $VERSION installed to $INSTALL_DIR/$BINARY"

MAN_DIR="${MAN_DIR:-/usr/local/share/man/man1}"
if ls "$TMP/man/"*.1 &>/dev/null 2>&1; then
  if [ ! -w "$(dirname "$MAN_DIR")" ] && [ ! -w "$MAN_DIR" ]; then
    echo "Installing man pages to $MAN_DIR (requires sudo)..."
    sudo mkdir -p "$MAN_DIR"
    sudo install -m 644 "$TMP/man/"*.1 "$MAN_DIR/"
  else
    mkdir -p "$MAN_DIR"
    install -m 644 "$TMP/man/"*.1 "$MAN_DIR/"
  fi
  echo "man pages installed to $MAN_DIR"
fi

echo ""
echo "Shell completion setup:"
echo "  bash:  $BINARY completion bash > /etc/bash_completion.d/$BINARY  # system-wide"
echo "  bash:  $BINARY completion bash > ~/.bash_completion               # user"
echo "  zsh:   $BINARY completion zsh > \"\${fpath[1]}/_$BINARY\""
echo "  fish:  $BINARY completion fish > ~/.config/fish/completions/$BINARY.fish"
