#!/bin/sh
set -e

REPO="urbangeeks/kollaber"
BINARY="kollaber"
INSTALL_DIR="/usr/local/bin"

# ---- detect OS ----
OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux"  ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# ---- detect arch ----
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)           ARCH="amd64" ;;
  aarch64 | arm64)  ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# ---- get latest version ----
VERSION="$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": *"\(.*\)".*/\1/')"

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version."
  exit 1
fi

# Strip leading 'v' for filename — goreleaser uses the numeric version in filenames
VERSION_NUM="${VERSION#v}"

FILENAME="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})…"

# ---- download and extract ----
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -sSfL "$URL" | tar -xz -C "$TMP"

# ---- install ----
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "Requesting sudo to install to $INSTALL_DIR"
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "Installed to $INSTALL_DIR/$BINARY"
echo "Run: kollaber --help"
