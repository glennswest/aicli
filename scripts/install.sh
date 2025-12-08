#!/bin/bash
# aicli installer for Linux and macOS

set -e

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="aicli"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

PLATFORM="${OS}-${ARCH}"

echo "Detected platform: $PLATFORM"
echo "Installing to: $INSTALL_DIR"

# Check if running from dist directory with binaries
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ -f "$SCRIPT_DIR/$PLATFORM/$BINARY_NAME" ]; then
    # Install from local dist
    echo "Installing from local build..."
    sudo cp "$SCRIPT_DIR/$PLATFORM/$BINARY_NAME" "$INSTALL_DIR/"
elif [ -f "$SCRIPT_DIR/../dist/$PLATFORM/$BINARY_NAME" ]; then
    # Install from project root
    sudo cp "$SCRIPT_DIR/../dist/$PLATFORM/$BINARY_NAME" "$INSTALL_DIR/"
else
    echo "Binary not found for $PLATFORM"
    echo "Please build first with: make $PLATFORM"
    exit 1
fi

sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"

# Sign on macOS
if [ "$OS" = "darwin" ]; then
    echo "Signing binary for macOS..."
    codesign --force --sign - "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || true
fi

echo "Installation complete!"
echo "Run 'aicli --init' to create default config"
echo "Run 'aicli --help' for usage"
