#!/bin/bash
# Coders installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/Jayphen/coders/main/packages/go/install.sh | bash

set -e

REPO="Jayphen/coders"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="coders"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    darwin|linux)
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

BINARY="coders-${OS}-${ARCH}"

echo "Installing coders for ${OS}/${ARCH}..."

# Get latest release version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "Failed to get latest version"
    exit 1
fi

echo "Latest version: $VERSION"

# Download binary
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"
echo "Downloading from: $DOWNLOAD_URL"

TEMP_FILE=$(mktemp)
curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_FILE"
chmod +x "$TEMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TEMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "Installing to ${INSTALL_DIR} requires sudo..."
    sudo mv "$TEMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "✅ coders installed successfully!"
echo ""
echo "Run 'coders --help' to get started."
echo ""
