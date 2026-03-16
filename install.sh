#!/bin/sh
set -e

BINARY="syllable"
REPO="asksyllable/syllable-cli"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# --- Detect OS ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  msys*|mingw*|cygwin*) OS="windows" ;;
  *) echo "Unsupported OS: $OS" && exit 1 ;;
esac

# --- Detect architecture ---
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# --- Resolve version ---
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: could not fetch latest version from GitHub"
    exit 1
  fi
fi

# Strip leading 'v' for filename, keep it for the tag
TAG="$VERSION"
VERSION_NUM="${VERSION#v}"

# --- Build download URL ---
BASE_URL="https://github.com/$REPO/releases/download/$TAG"
if [ "$OS" = "windows" ]; then
  FILENAME="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.zip"
else
  FILENAME="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
fi
URL="$BASE_URL/$FILENAME"
CHECKSUM_URL="$BASE_URL/${BINARY}_${VERSION_NUM}_checksums.txt"

echo "Installing $BINARY $VERSION ($OS/$ARCH)..."

# --- Download ---
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$FILENAME"

# --- Verify checksum ---
CHECKSUM_FILE="$TMP/checksums.txt"
curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_FILE"

cd "$TMP"
if command -v sha256sum >/dev/null 2>&1; then
  grep "$FILENAME" "$CHECKSUM_FILE" | sha256sum -c --quiet
elif command -v shasum >/dev/null 2>&1; then
  grep "$FILENAME" "$CHECKSUM_FILE" | shasum -a 256 -c --quiet
else
  echo "Warning: could not verify checksum (sha256sum/shasum not found)"
fi
cd - >/dev/null

# --- Extract ---
if [ "$OS" = "windows" ]; then
  unzip -q "$TMP/$FILENAME" -d "$TMP"
else
  tar -xzf "$TMP/$FILENAME" -C "$TMP"
fi

# --- Install ---
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
  echo "Note: /usr/local/bin not writable, installing to $INSTALL_DIR"
  echo "Make sure $INSTALL_DIR is in your PATH."
fi

mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "✓ $BINARY $VERSION installed to $INSTALL_DIR/$BINARY"
echo ""
echo "Run 'syllable --help' to get started."
