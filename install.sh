#!/bin/sh
set -e

REPO="XBS-Nathan/Nova"
BINARY="nova"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  *)       echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET="${BINARY}-${OS}-${ARCH}"
echo "Downloading ${ASSET}..."

URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

TMPFILE="$(mktemp)"
trap 'rm -f "$TMPFILE"' EXIT

if ! curl -fsSL -o "$TMPFILE" "$URL"; then
  echo "Failed to download ${URL}"
  exit 1
fi

chmod +x "$TMPFILE"

if [ -w "$INSTALL_DIR" ] || [ "$(id -u)" = "0" ]; then
  mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
  echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
elif command -v sudo >/dev/null 2>&1; then
  sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
  echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
  echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "Add ${INSTALL_DIR} to your PATH" ;;
  esac
fi

echo "Run 'nova --help' to get started"
